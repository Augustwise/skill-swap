"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";

type University = {
  id: string;
  name: string;
  emailDomain: string;
};

type ApiProblem = {
  error?: {
    code?: string;
    fields?: Record<string, string>;
  };
};

const asset = "/figma/register";

function translateFieldErrors(fields: Record<string, string>) {
  const translated: Record<string, string> = {};
  for (const [field, message] of Object.entries(fields)) {
    if (field === "email") {
      translated.email = message.includes("not registered")
        ? "Цей університет ще не зареєстрований у SkillSwap."
        : message.includes("valid")
          ? "Введи коректну адресу пошти."
          : "Використай університетську пошту.";
    } else if (field === "password") {
      translated.password = message.includes("72")
        ? "Пароль не може перевищувати 72 байти."
        : "Пароль має містити щонайменше 12 символів.";
    } else if (field === "firstName") {
      translated.firstName = "Введи ім’я довжиною до 100 символів.";
    } else if (field === "lastName") {
      translated.lastName = "Введи прізвище довжиною до 100 символів.";
    }
  }
  return translated;
}

export function RegistrationForm() {
  const [universities, setUniversities] = useState<University[]>([]);
  const [universityError, setUniversityError] = useState("");
  const [universityId, setUniversityId] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [agreed, setAgreed] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [registeredEmail, setRegisteredEmail] = useState("");
  const [verificationEmailSent, setVerificationEmailSent] = useState(true);

  const clearFieldError = (field: string) => {
    if (fieldErrors[field]) {
      setFieldErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    }
  };

  useEffect(() => {
    const controller = new AbortController();

    async function loadUniversities() {
      try {
        const response = await fetch("/api/v1/universities", { signal: controller.signal });
        if (!response.ok) throw new Error("University catalog unavailable");
        const data: { items?: University[] } = await response.json();
        setUniversities(data.items ?? []);
        if (!data.items?.length) setUniversityError("Список університетів поки порожній.");
      } catch {
        if (!controller.signal.aborted) {
          setUniversityError("Не вдалося завантажити університети. Спробуй оновити сторінку.");
        }
      }
    }

    void loadUniversities();
    return () => controller.abort();
  }, []);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setFieldErrors({});

    const form = new FormData(event.currentTarget);
    const firstName = String(form.get("firstName") ?? "").trim();
    const lastName = String(form.get("lastName") ?? "").trim();
    const email = String(form.get("email") ?? "")
      .trim()
      .toLowerCase();
    const password = String(form.get("password") ?? "");
    const agreement = agreed || form.get("agreement") === "on";
    const university = universities.find((item) => item.id === universityId);

    const errors: Record<string, string> = {};

    if (!firstName) {
      errors.firstName = "Введи ім’я.";
    } else if (firstName.length > 100) {
      errors.firstName = "Введи ім’я довжиною до 100 символів.";
    }

    if (!lastName) {
      errors.lastName = "Введи прізвище.";
    } else if (lastName.length > 100) {
      errors.lastName = "Введи прізвище довжиною до 100 символів.";
    }

    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!email) {
      errors.email = "Введи університетську пошту.";
    } else if (!emailPattern.test(email)) {
      errors.email = "Введи коректну адресу пошти.";
    } else if (university && email.split("@")[1] !== university.emailDomain.toLowerCase()) {
      errors.email = `Пошта має належати домену ${university.emailDomain}.`;
    }

    if (!universityId) {
      errors.university = "Обери університет зі списку.";
    }

    if (!password) {
      errors.password = "Введи пароль.";
    } else if (password.length < 12) {
      errors.password = "Пароль має містити щонайменше 12 символів.";
    } else if (new TextEncoder().encode(password).length > 72) {
      errors.password = "Пароль не може перевищувати 72 байти.";
    }

    if (!agreement) {
      errors.agreement = "Погодься з умовами використання, щоб продовжити.";
    }

    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      const formEl = event.currentTarget;
      const order = ["firstName", "lastName", "email", "university", "password", "agreement"];
      const firstField = order.find((f) => errors[f]);
      if (firstField) {
        const el = formEl.elements.namedItem(firstField) as HTMLElement | null;
        el?.focus();
      }
      return;
    }

    setPending(true);
    try {
      const response = await fetch("/api/v1/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ firstName, lastName, email, password }),
      });
      const data: ApiProblem & { verificationEmailSent?: boolean } = await response.json();

      if (!response.ok) {
        if (response.status === 409) {
          setFieldErrors({ email: "Акаунт із цією поштою вже існує." });
        } else if (data.error?.fields) {
          setFieldErrors(translateFieldErrors(data.error.fields));
        } else {
          setError("Не вдалося створити акаунт. Спробуй ще раз.");
        }
        return;
      }

      setRegisteredEmail(email);
      setVerificationEmailSent(data.verificationEmailSent !== false);
    } catch {
      setError("Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.");
    } finally {
      setPending(false);
    }
  }

  if (registeredEmail) {
    return (
      <div className="register-form register-success" role="status">
        <h1 id="register-title">Акаунт створено</h1>
        <p>
          {verificationEmailSent
            ? `Ми надіслали лист для підтвердження на ${registeredEmail}. Перевір пошту, щоб завершити реєстрацію.`
            : `Акаунт для ${registeredEmail} створено, але лист не вдалося надіслати. Увійди й надішли підтвердження повторно.`}
        </p>
        <Link className="register-submit register-success__link" href="/login">
          Перейти до входу
        </Link>
      </div>
    );
  }

  return (
    <form className="register-form" noValidate onSubmit={handleSubmit}>
      <div className="register-form__heading">
        <h1 id="register-title">Створи акаунт</h1>
        <p>Реєстрація займе менше хвилини</p>
      </div>

      <div className="register-form__name-row">
        <label className="register-field">
          <span>Ім&apos;я</span>
          <input
            name="firstName"
            type="text"
            placeholder="Андрій"
            autoComplete="given-name"
            maxLength={100}
            aria-invalid={Boolean(fieldErrors.firstName)}
            onChange={() => clearFieldError("firstName")}
            required
          />
          {fieldErrors.firstName && (
            <small className="register-field__error">{fieldErrors.firstName}</small>
          )}
        </label>
        <label className="register-field">
          <span>Прізвище</span>
          <input
            name="lastName"
            type="text"
            placeholder="Мельник"
            autoComplete="family-name"
            maxLength={100}
            aria-invalid={Boolean(fieldErrors.lastName)}
            onChange={() => clearFieldError("lastName")}
            required
          />
          {fieldErrors.lastName && (
            <small className="register-field__error">{fieldErrors.lastName}</small>
          )}
        </label>
      </div>

      <label className="register-field">
        <span>Університетська пошта</span>
        <input
          name="email"
          type="email"
          placeholder="andrii.melnyk@edu.kpi.ua"
          autoComplete="email"
          maxLength={320}
          aria-invalid={Boolean(fieldErrors.email)}
          onChange={() => clearFieldError("email")}
          required
        />
        {fieldErrors.email && <small className="register-field__error">{fieldErrors.email}</small>}
      </label>

      <label className="register-field register-field--select">
        <span>Університет</span>
        <select
          name="university"
          value={universityId}
          onChange={(event) => {
            setUniversityId(event.target.value);
            clearFieldError("university");
            if (fieldErrors.email?.includes("Пошта має належати домену")) {
              clearFieldError("email");
            }
          }}
          aria-invalid={Boolean(fieldErrors.university)}
          required
        >
          <option value="" disabled>
            Обери університет
          </option>
          {universities.map((university) => (
            <option key={university.id} value={university.id}>
              {university.name}
            </option>
          ))}
        </select>
        <Image src={`${asset}/chevron-down.svg`} alt="" width={16} height={16} />
        {(fieldErrors.university || universityError) && (
          <small className="register-field__error">
            {fieldErrors.university || universityError}
          </small>
        )}
      </label>

      <div className="register-field register-field--password">
        <label htmlFor="register-password">Пароль</label>
        <input
          id="register-password"
          name="password"
          type={showPassword ? "text" : "password"}
          placeholder="Мінімум 12 символів"
          autoComplete="new-password"
          minLength={12}
          aria-invalid={Boolean(fieldErrors.password)}
          onChange={() => clearFieldError("password")}
          required
        />
        <button
          type="button"
          className="register-field__eye"
          onClick={() => setShowPassword((value) => !value)}
          aria-label={showPassword ? "Приховати пароль" : "Показати пароль"}
          aria-pressed={showPassword}
        >
          <Image
            src={`${asset}/${showPassword ? "eye-off" : "eye"}.svg`}
            alt=""
            width={17}
            height={17}
          />
        </button>
        {fieldErrors.password && (
          <small className="register-field__error">{fieldErrors.password}</small>
        )}
      </div>

      <div className="register-form__notice">
        <Image src={`${asset}/cap.svg`} alt="" width={17} height={17} />
        <p>
          Потрібна саме університетська пошта — так ми підтверджуємо, що ти студент, і показуємо
          значок «Верифіковано».
        </p>
      </div>

      <div className="register-form__agreement-wrap">
        <label className="register-form__agreement">
          <input
            className="checkbox"
            name="agreement"
            type="checkbox"
            checked={agreed}
            onChange={(event) => {
              setAgreed(event.target.checked);
              clearFieldError("agreement");
            }}
            aria-invalid={Boolean(fieldErrors.agreement)}
            required
          />
          <span>Погоджуюся з умовами використання</span>
        </label>
        {fieldErrors.agreement && (
          <small className="register-field__error">{fieldErrors.agreement}</small>
        )}
      </div>

      {error && (
        <p className="register-form__error" role="alert">
          {error}
        </p>
      )}
      <button className="register-submit" type="submit" disabled={pending}>
        {pending ? "Створюємо акаунт…" : "Створити акаунт"}
      </button>
      <p className="register-form__login">
        Уже маєш акаунт? <Link href="/login">Увійти</Link>
      </p>
    </form>
  );
}
