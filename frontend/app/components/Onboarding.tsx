"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import {
  OnboardingError,
  onboardingRequest,
  profileFields,
  selectedSkills,
  type Profile,
  type ProfileFields,
  type Reference,
  type SelectedSkill,
  type Skill,
} from "./OnboardingApi";
import { OnboardingProfile } from "./OnboardingProfile";
import { OnboardingIcon as Icon, OnboardingSkills } from "./OnboardingSkills";

const steps = ["Профіль", "Що я вмію", "Що хочу вивчити", "Формат"];
const fieldMessages: Record<string, string> = {
  firstName: "Введи ім’я довжиною до 100 символів.",
  lastName: "Введи прізвище довжиною до 100 символів.",
  facultyId: "Обери факультет свого університету.",
  course: "Обери курс від 1 до 6.",
  city: "Вкажи місто довжиною до 100 символів. Для офлайн-занять місто обов’язкове.",
  bio: "Опис має містити до 600 символів.",
};

export function Onboarding() {
  const router = useRouter();
  const [status, setStatus] = useState<
    "loading" | "ready" | "login" | "error" | "verificationError"
  >("loading");
  const verification = useRef<Promise<unknown> | null>(null);
  const [verified, setVerified] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [csrfToken, setCsrfToken] = useState("");
  const [profile, setProfile] = useState<Profile | null>(null);
  const [fields, setFields] = useState<ProfileFields>({
    firstName: "",
    lastName: "",
    facultyId: "",
    course: "",
    city: "",
    bio: "",
  });
  const [faculties, setFaculties] = useState<Reference[]>([]);
  const [categories, setCategories] = useState<Reference[]>([]);
  const [catalog, setCatalog] = useState<Skill[]>([]);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState("");
  const [catalogAttempt, setCatalogAttempt] = useState(0);
  const [category, setCategory] = useState("");
  const [query, setQuery] = useState("");
  const [teaching, setTeaching] = useState<SelectedSkill[]>([]);
  const [learning, setLearning] = useState<SelectedSkill[]>([]);
  const [step, setStep] = useState(0);
  const [format, setFormat] = useState("");
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [sessionExpired, setSessionExpired] = useState(false);
  const saving = useRef(false);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      const token = new URLSearchParams(window.location.search).get("token");
      if (token) {
        // Email tokens are single-use; share the request across Strict Mode effect replays.
        verification.current ??= onboardingRequest("auth/verify-email", {
          method: "POST",
          body: JSON.stringify({ token }),
        });
        try {
          await verification.current;
          if (controller.signal.aborted) return;
          const url = new URL(window.location.href);
          url.searchParams.delete("token");
          window.history.replaceState(null, "", url.pathname + url.search);
          setVerified(true);
        } catch {
          if (!controller.signal.aborted) setStatus("verificationError");
          return;
        }
      }
      try {
        const session = await onboardingRequest<{ csrfToken: string }>("auth/me", {
          signal: controller.signal,
        });
        const [data, categoryData] = await Promise.all([
          onboardingRequest<{ profile: Profile }>("me/profile", { signal: controller.signal }),
          onboardingRequest<{ items: Reference[] }>("skill-categories", {
            signal: controller.signal,
          }),
        ]);
        const facultyData = await onboardingRequest<{ items: Reference[] }>(
          `universities/${data.profile.university.id}/faculties`,
          { signal: controller.signal },
        );
        if (controller.signal.aborted) return;
        setCsrfToken(session.csrfToken);
        setProfile(data.profile);
        setFields(profileFields(data.profile));
        setTeaching(selectedSkills(data.profile.teachingSkills));
        setLearning(selectedSkills(data.profile.learningSkills));
        setFormat(data.profile.formats.length === 2 ? "both" : (data.profile.formats[0] ?? ""));
        setCategories(categoryData.items);
        setFaculties(facultyData.items);
        setStatus("ready");
      } catch (error) {
        if (!controller.signal.aborted)
          setStatus(error instanceof OnboardingError && error.status === 401 ? "login" : "error");
      }
    }
    void load();
    return () => controller.abort();
  }, [attempt]);

  useEffect(() => {
    if (status !== "ready" || (step !== 1 && step !== 2)) return;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setCatalogLoading(true);
      setCatalogError("");
      try {
        const data = await onboardingRequest<{ items: Skill[] }>(
          `skills?limit=100&q=${encodeURIComponent(query.trim())}`,
          { signal: controller.signal },
        );
        if (!controller.signal.aborted) setCatalog(data.items);
      } catch {
        if (!controller.signal.aborted) setCatalogError("Не вдалося завантажити навички.");
      } finally {
        if (!controller.signal.aborted) setCatalogLoading(false);
      }
    }, 200);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [status, step, query, catalogAttempt]);

  function changeStep(next: number) {
    setNotice("");
    setErrors({});
    setQuery("");
    setCategory("");
    setCatalog([]);
    setCatalogError("");
    setCatalogLoading(next === 1 || next === 2);
    setStep(next);
    window.requestAnimationFrame(() => document.getElementById("onboarding-title")?.focus());
  }

  function changeField(field: keyof ProfileFields, value: string) {
    setFields((current) => ({ ...current, [field]: value }));
    setErrors((current) => ({ ...current, [field]: "" }));
    setNotice("");
  }

  async function saveSkills(list: "teaching" | "learning", selected: SelectedSkill[]) {
    // Reconcile against the server so a retry is safe after a partially successful save.
    let { profile: current } = await onboardingRequest<{ profile: Profile }>("me/profile");
    const existing = current[list === "teaching" ? "teachingSkills" : "learningSkills"];
    for (const skill of existing) {
      if (!selected.some((item) => item.id === skill.skillId)) {
        ({ profile: current } = await onboardingRequest<{ profile: Profile }>(
          `me/${list}-skills/${skill.skillId}`,
          { method: "DELETE" },
          csrfToken,
        ));
      }
    }
    for (const skill of selected) {
      const previous = existing.find((item) => item.skillId === skill.id);
      if (previous?.level === skill.level) continue;
      ({ profile: current } = await onboardingRequest<{ profile: Profile }>(
        `me/${list}-skills${previous ? `/${skill.id}` : ""}`,
        {
          method: previous ? "PATCH" : "POST",
          body: JSON.stringify(
            previous ? { level: skill.level } : { skillId: skill.id, level: skill.level },
          ),
        },
        csrfToken,
      ));
    }
    setProfile(current);
  }

  async function handleNext(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving.current) return;
    setNotice("");
    setErrors({});
    if (step === 0) {
      const invalid: Record<string, string> = {};
      for (const field of ["firstName", "lastName"] as const) {
        if (!fields[field].trim()) invalid[field] = fieldMessages[field];
      }
      if (Object.keys(invalid).length) {
        setErrors(invalid);
        (event.currentTarget.elements.namedItem(Object.keys(invalid)[0]) as HTMLElement)?.focus();
        return;
      }
    }
    if ((step === 1 && !teaching.length) || (step === 2 && !learning.length)) {
      setNotice("Обери хоча б одну навичку.");
      return;
    }
    if ((step === 1 && teaching.length > 5) || (step === 2 && learning.length > 5)) {
      setNotice("Залиш не більше 5 навичок, щоб продовжити.");
      return;
    }
    if (step === 3 && !format) {
      setNotice("Обери формат зустрічей.");
      return;
    }
    saving.current = true;
    setPending(true);
    try {
      if (step === 0 || step === 3) {
        const body =
          step === 0
            ? {
                ...fields,
                firstName: fields.firstName.trim(),
                lastName: fields.lastName.trim(),
                course: fields.course ? Number(fields.course) : null,
                facultyId: fields.facultyId || null,
              }
            : { formats: format === "both" ? ["ONLINE", "OFFLINE"] : [format] };
        const data = await onboardingRequest<{ profile: Profile }>(
          "me/profile",
          { method: "PATCH", body: JSON.stringify(body) },
          csrfToken,
        );
        setProfile(data.profile);
        setFields(profileFields(data.profile));
      } else
        await saveSkills(step === 1 ? "teaching" : "learning", step === 1 ? teaching : learning);
      if (step < 3) changeStep(step + 1);
      else router.push("/");
    } catch (error) {
      if (error instanceof OnboardingError) {
        setSessionExpired(error.status === 401);
        setErrors(
          Object.fromEntries(
            Object.keys(error.fields).map((field) => [
              field,
              fieldMessages[field] ?? "Перевір значення цього поля.",
            ]),
          ),
        );
        setNotice(error.status === 422 ? "Перевір дані перед продовженням." : error.message);
      } else setNotice("Не вдалося зв’язатися із сервером. Спробуй ще раз.");
    } finally {
      saving.current = false;
      setPending(false);
    }
  }

  if (status !== "ready") {
    return (
      <main className="onboarding-page onboarding-page--message">
        <div className="onboarding-message" role={status === "loading" ? "status" : "alert"}>
          <Image src="/figma/onboarding/mark.svg" alt="" width={30} height={30} />
          <h1>
            {status === "loading"
              ? "Завантажуємо профіль…"
              : status === "login"
                ? verified
                  ? "Пошту підтверджено"
                  : "Увійди, щоб продовжити"
                : status === "verificationError"
                  ? "Посилання не працює"
                  : "Не вдалося завантажити профіль"}
          </h1>
          <p>
            {status === "loading"
              ? "Зачекай, поки ми підготуємо твої дані."
              : status === "login"
                ? "Увійди в акаунт, щоб заповнити профіль і додати навички."
                : status === "verificationError"
                  ? "Посилання недійсне або вже використане. Увійди в акаунт, щоб продовжити."
                  : "Перевір з’єднання із сервером і спробуй ще раз."}
          </p>
          {status === "error" ? (
            <button
              type="button"
              className="onboarding-button onboarding-button--secondary"
              onClick={() => {
                setStatus("loading");
                setAttempt((value) => value + 1);
              }}
            >
              Спробувати ще раз
            </button>
          ) : (
            status !== "loading" && <Link href="/login?next=/onboarding">Перейти до входу</Link>
          )}
        </div>
      </main>
    );
  }

  return (
    <main className="onboarding-page">
      <header className="onboarding-header">
        <Link className="onboarding-logo" href="/" aria-label="SkillSwap — головна">
          <Image src="/figma/onboarding/mark.svg" alt="" width={30} height={30} />
          <span>SkillSwap</span>
        </Link>
        <span className="onboarding-header__step">Крок {step + 1} з 4</span>
        <Link
          className={`onboarding-skip ${pending ? "is-disabled" : ""}`}
          href="/"
          aria-disabled={pending}
          onClick={(event) => {
            if (pending) event.preventDefault();
          }}
        >
          Пропустити
        </Link>
      </header>
      <div className="onboarding-layout">
        <form
          className="onboarding-card"
          aria-labelledby="onboarding-title"
          aria-busy={pending}
          noValidate
          onSubmit={handleNext}
        >
          <ol className="onboarding-progress" aria-label="Етапи онбордингу">
            {steps.map((label, index) => {
              const state = index < step ? "complete" : index === step ? "active" : "future";
              return (
                <li
                  className={`onboarding-progress__item onboarding-progress__item--${state}`}
                  key={label}
                  aria-current={index === step ? "step" : undefined}
                >
                  <span className="onboarding-progress__circle">
                    {state === "complete" ? <Icon name="check" size={14} /> : index + 1}
                  </span>
                  <span className="onboarding-progress__label">{label}</span>
                  {index < 3 && <span className="onboarding-progress__line" aria-hidden="true" />}
                </li>
              );
            })}
          </ol>
          <fieldset className="onboarding-fields" disabled={pending || sessionExpired}>
            <legend className="sr-only">{steps[step]}</legend>
            {step === 0 ? (
              <OnboardingProfile
                values={fields}
                university={profile?.university.name ?? ""}
                faculties={faculties}
                errors={errors}
                onChange={changeField}
              />
            ) : step === 1 || step === 2 ? (
              <OnboardingSkills
                teaching={step === 1}
                categories={categories}
                category={category}
                query={query}
                skills={catalog}
                loading={catalogLoading}
                catalogError={catalogError}
                selected={step === 1 ? teaching : learning}
                onCategory={setCategory}
                onQuery={(value) => {
                  setQuery(value);
                  setCatalogLoading(true);
                }}
                onChange={step === 1 ? setTeaching : setLearning}
                onNotice={setNotice}
                onRetry={() => setCatalogAttempt((value) => value + 1)}
              />
            ) : (
              <>
                <div className="onboarding-intro">
                  <h1 id="onboarding-title" tabIndex={-1}>
                    Як тобі зручно навчатися?
                  </h1>
                  <p>Обери формат зустрічей для обміну навичками.</p>
                </div>
                <div className="onboarding-formats" role="group" aria-label="Формат зустрічей">
                  {[
                    { value: "ONLINE", label: "Онлайн" },
                    { value: "OFFLINE", label: "Офлайн" },
                    { value: "both", label: "Обидва формати" },
                  ].map((item) => (
                    <button
                      type="button"
                      className={`onboarding-format ${format === item.value ? "is-selected" : ""}`}
                      aria-pressed={format === item.value}
                      onClick={() => {
                        setFormat(item.value);
                        setNotice("");
                      }}
                      key={item.value}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
                {errors.city && (
                  <p className="onboarding-notice">
                    {errors.city} Повернися до кроку «Профіль», щоб додати місто.
                  </p>
                )}
              </>
            )}
            <div className="onboarding-actions">
              {step === 0 ? (
                <Link
                  className="onboarding-button onboarding-button--secondary"
                  href="/"
                  onClick={(event) => {
                    if (pending) event.preventDefault();
                  }}
                >
                  Назад
                </Link>
              ) : (
                <button
                  type="button"
                  className="onboarding-button onboarding-button--secondary"
                  onClick={() => changeStep(step - 1)}
                >
                  Назад
                </button>
              )}
              <button type="submit" className="onboarding-button onboarding-button--primary">
                <Icon name="arrow-right" size={18} />
                {pending
                  ? "Зберігаємо…"
                  : [
                      "Далі — додати навички",
                      "Далі — що хочу вивчити",
                      "Далі — обрати формат",
                      "Завершити",
                    ][step]}
              </button>
            </div>
          </fieldset>
          {notice && (
            <p className="onboarding-notice" role="alert">
              {notice}
            </p>
          )}
          {sessionExpired && <Link href="/login?next=/onboarding">Увійти знову</Link>}
        </form>
        <aside className="onboarding-sidebar" aria-label="Підказки для онбордингу">
          <div className="onboarding-matches">
            <div className="onboarding-matches__heading">
              <span className="onboarding-matches__icon">
                <Icon name="sparkles" size={16} />
              </span>
              <h2>
                {step === 0
                  ? "Почни зі знайомства"
                  : step === 1
                    ? "Кожен має чим поділитися"
                    : "Збіги знаходяться в розробці"}
              </h2>
            </div>
            <p>
              {step === 0
                ? "Твій профіль допоможе іншим студентам дізнатися більше про тебе та знайти спільні інтереси."
                : step === 1
                  ? "Не обов’язково бути експертом. Твій досвід може стати чиїмось першим кроком до нової навички."
                  : "Ми працюємо над розумним підбором студентів на основі взаємних навичок."}
            </p>
            <div className="onboarding-matches__placeholder">
              {step === 0
                ? "Додай кілька слів про себе — з цього починається обмін знаннями."
                : step === 1
                  ? "Вкажи свій рівень чесно, щоб підібрати комфортний обмін."
                  : "Тут зʼявляться персональні рекомендації, щойно алгоритм підбору буде запущено."}
            </div>
          </div>
          <div className="onboarding-tip">
            <span className="onboarding-tip__icon">
              <Icon name="award" size={16} />
            </span>
            <div>
              <h2>Порада</h2>
              <p>
                {step === 0
                  ? "Розкажи, що тебе цікавить. Місто знадобиться, якщо плануєш зустрічатися офлайн."
                  : "Обирай 2–3 конкретні навички замість однієї загальної — так збігів буде більше."}
              </p>
            </div>
          </div>
        </aside>
      </div>
      {verified && (
        <span className="sr-only" role="status">
          Пошту підтверджено.
        </span>
      )}
    </main>
  );
}
