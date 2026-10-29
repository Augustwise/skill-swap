"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { AuthError, authRequest, requestError } from "./AuthApi";
import {
  emailError,
  focusInvalid,
  normalizeEmail,
  passwordError,
  registrationErrors,
} from "./AuthValidation";

type University = {
  id: string;
  name: string;
  emailDomain: string;
};

const asset = "/figma/register";

export function RegistrationForm() {
  const [universities, setUniversities] = useState<University[]>([]);
  const [universityError, setUniversityError] = useState("");
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [catalogAttempt, setCatalogAttempt] = useState(0);
  const [universityId, setUniversityId] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [agreed, setAgreed] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [registeredEmail, setRegisteredEmail] = useState("");
  const [verificationEmailSent, setVerificationEmailSent] = useState(true);
  const submitting = useRef(false);

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
        const data = await authRequest<{ items: University[] }>("universities", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) return;
        setUniversities(data.items ?? []);
        setUniversityError(data.items?.length ? "" : "Список університетів поки порожній.");
      } catch {
        if (!controller.signal.aborted) {
          setUniversityError("Не вдалося завантажити університети. Спробуй ще раз.");
        }
      } finally {
        if (!controller.signal.aborted) setCatalogLoading(false);
      }
    }

    void loadUniversities();
    return () => controller.abort();
  }, [catalogAttempt]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    const formElement = event.currentTarget;
    setError("");
    setFieldErrors({});

    const form = new FormData(event.currentTarget);
    const firstName = String(form.get("firstName") ?? "").trim();
    const lastName = String(form.get("lastName") ?? "").trim();
    const email = normalizeEmail(String(form.get("email") ?? ""));
    const password = String(form.get("password") ?? "");
    const agreement = agreed || form.get("agreement") === "on";
    const university = universities.find((item) => item.id === universityId);

    const errors: Record<string, string> = {};

    if (!firstName) {
      errors.firstName = "Введи ім’я.";
    } else if (Array.from(firstName).length > 100) {
      errors.firstName = "Введи ім’я довжиною до 100 символів.";
    }

    if (!lastName) {
      errors.lastName = "Введи прізвище.";
    } else if (Array.from(lastName).length > 100) {
      errors.lastName = "Введи прізвище довжиною до 100 символів.";
    }

    const invalidEmail = emailError(email);
    if (invalidEmail) errors.email = invalidEmail;
    else if (university && email.split("@")[1] !== university.emailDomain.toLowerCase()) {
      errors.email = `Пошта має належати домену ${university.emailDomain}.`;
    }

    if (!university || catalogLoading || universityError) {
      errors.university = "Обери університет зі списку.";
    }

    const invalidPassword = passwordError(password);
    if (invalidPassword) errors.password = invalidPassword;

    if (!agreement) {
      errors.agreement = "Погодься з умовами використання, щоб продовжити.";
    }

    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      focusInvalid(formElement, errors);
      return;
    }

    submitting.current = true;
    setPending(true);
    try {
      const data = await authRequest<{ verificationEmailSent: boolean }>("auth/register", {
        method: "POST",
        body: JSON.stringify({ firstName, lastName, email, password }),
      });
      formElement.reset();
      setRegisteredEmail(email);
      setVerificationEmailSent(data.verificationEmailSent !== false);
    } catch (error) {
      const errors =
        error instanceof AuthError
          ? error.code === "email_taken"
            ? { email: "Акаунт із цією поштою вже існує." }
            : registrationErrors(error.fields)
          : {};
      setFieldErrors(errors);
      focusInvalid(formElement, errors);
      setError(Object.keys(errors).length ? "Перевір виділені поля." : requestError(error));
    } finally {
      submitting.current = false;
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
    <form className="register-form" noValidate onSubmit={handleSubmit} aria-busy={pending}>
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
            aria-describedby={fieldErrors.firstName ? "register-firstName-error" : undefined}
            onChange={() => clearFieldError("firstName")}
            required
          />
          {fieldErrors.firstName && (
            <small id="register-firstName-error" className="register-field__error">
              {fieldErrors.firstName}
            </small>
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
            aria-describedby={fieldErrors.lastName ? "register-lastName-error" : undefined}
            onChange={() => clearFieldError("lastName")}
            required
          />
          {fieldErrors.lastName && (
            <small id="register-lastName-error" className="register-field__error">
              {fieldErrors.lastName}
            </small>
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
          aria-describedby={fieldErrors.email ? "register-email-error" : undefined}
          onChange={() => clearFieldError("email")}
          required
        />
        {fieldErrors.email && (
          <small id="register-email-error" className="register-field__error">
            {fieldErrors.email}
          </small>
        )}
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
          aria-describedby="register-university-hint"
          disabled={catalogLoading || pending}
          required
        >
          <option value="" disabled>
            {catalogLoading ? "Завантажуємо університети…" : "Обери університет"}
          </option>
          {universities.map((university) => (
            <option key={university.id} value={university.id}>
              {university.name}
            </option>
          ))}
        </select>
        <Image src={`${asset}/chevron-down.svg`} alt="" width={16} height={16} />
        <small
          id="register-university-hint"
          className={
            fieldErrors.university || universityError ? "register-field__error" : "auth-hint"
          }
        >
          {fieldErrors.university ||
            universityError ||
            (universityId
              ? `Домен пошти: ${universities.find((item) => item.id === universityId)?.emailDomain ?? ""}`
              : "Обери університет, до якого належить твоя пошта.")}
        </small>
      </label>
      {universityError && (
        <button
          className="login-form__back"
          type="button"
          disabled={catalogLoading}
          onClick={() => {
            setCatalogLoading(true);
            setCatalogAttempt((value) => value + 1);
          }}
        >
          Завантажити університети ще раз
        </button>
      )}

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
          aria-describedby="register-password-hint"
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
        <small
          id="register-password-hint"
          className={fieldErrors.password ? "register-field__error" : "auth-hint"}
        >
          {fieldErrors.password || "Щонайменше 12 символів."}
        </small>
      </div>

      <div className="register-form__notice">
        <Image src={`${asset}/cap.svg`} alt="" width={17} height={17} />
        <p>
          Використай пошту свого університету. Після реєстрації надішлемо посилання для
          підтвердження адреси.
        </p>
      </div>

      <div className="register-form__agreement-wrap">
        <div className="register-form__agreement">
          <input
            id="register-agreement"
            aria-labelledby="register-agreement-text"
            className="checkbox"
            name="agreement"
            type="checkbox"
            checked={agreed}
            onChange={(event) => {
              setAgreed(event.target.checked);
              clearFieldError("agreement");
            }}
            aria-invalid={Boolean(fieldErrors.agreement)}
            aria-describedby={fieldErrors.agreement ? "register-agreement-error" : undefined}
            required
          />
          <span id="register-agreement-text">
            <label htmlFor="register-agreement">Погоджуюся з </label>
            <Link href="/terms" target="_blank" rel="noopener noreferrer">
              умовами використання та обробки даних
              <span className="sr-only"> (відкриється в новій вкладці)</span>
            </Link>
          </span>
        </div>
        {fieldErrors.agreement && (
          <small id="register-agreement-error" className="register-field__error">
            {fieldErrors.agreement}
          </small>
        )}
      </div>

      {error && (
        <p className="register-form__error" role="alert">
          {error}
        </p>
      )}
      <button
        className="register-submit"
        type="submit"
        disabled={pending || catalogLoading || Boolean(universityError)}
      >
        {pending ? "Створюємо акаунт…" : "Створити акаунт"}
      </button>
      <p className="register-form__login">
        Уже маєш акаунт? <Link href="/login">Увійти</Link>
      </p>
    </form>
  );
}
