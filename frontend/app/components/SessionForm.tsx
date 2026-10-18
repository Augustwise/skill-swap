"use client";

import { useId, useRef, useState, type FormEvent } from "react";
import { AppDialog } from "./AppDialog";
import { AuthError } from "./AuthApi";
import { loginHref } from "./AppRoutes";
import Link from "next/link";
import { fullName } from "./DiscoveryFormat";
import {
  proposeSession,
  sessionErrorMessage,
  sessionHref,
  type LessonSession,
  type SessionDraft,
} from "./SessionApi";
import { localDateTime, sessionDraftErrors, sessionPayload } from "./SessionTime";

export function SessionForm({
  lesson,
  csrfToken,
  onClose,
  onSaved,
}: {
  lesson: LessonSession;
  csrfToken: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const id = useId();
  const busy = useRef(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<unknown>(null);
  const [errors, setErrors] = useState<Partial<Record<keyof SessionDraft, string>>>({});
  const [draft, setDraft] = useState<SessionDraft>({
    title: lesson.title,
    description: lesson.description ?? "",
    teacherId: lesson.teacher.id,
    startsAt: localDateTime(lesson.startsAt),
    endsAt: localDateTime(lesson.endsAt),
    format: lesson.format,
    meetingPlace: lesson.meetingPlace ?? "",
    meetingUrl: lesson.meetingUrl ?? "",
  });

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy.current) return;
    const invalid = sessionDraftErrors(draft, [lesson.teacher.id]);
    setErrors(invalid);
    if (Object.keys(invalid).length) {
      event.currentTarget
        .querySelector<HTMLElement>(`[name="${Object.keys(invalid)[0]}"]`)
        ?.focus();
      return;
    }
    busy.current = true;
    setPending(true);
    setFailure(null);
    try {
      await proposeSession(lesson.id, sessionPayload(draft), csrfToken);
      onSaved();
    } catch (error) {
      setFailure(error);
      if (error instanceof AuthError && error.code === "validation_failed") {
        setErrors(
          Object.fromEntries(
            Object.keys(error.fields).map((key) => [key, "Перевір значення цього поля."]),
          ),
        );
      }
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  function field(
    name: "title" | "startsAt" | "endsAt" | "meetingUrl" | "meetingPlace",
    label: string,
    type = "text",
    maxLength?: number,
  ) {
    return (
      <label className="session-field" htmlFor={`${id}-${name}`}>
        {label}
        <input
          id={`${id}-${name}`}
          name={name}
          type={type}
          value={draft[name]}
          maxLength={maxLength}
          aria-invalid={Boolean(errors[name])}
          aria-describedby={errors[name] ? `${id}-${name}-error` : undefined}
          onChange={(event) => setDraft({ ...draft, [name]: event.target.value })}
        />
        {errors[name] && (
          <span className="exchange-field__error" id={`${id}-${name}-error`}>
            {errors[name]}
          </span>
        )}
      </label>
    );
  }

  return (
    <AppDialog
      title={lesson.status === "DRAFT" ? "Запланувати заняття" : "Запропонувати інший час"}
      description="Партнер має погодити пропозицію. До погодження час не резервується."
      className="session-dialog"
      canClose={!pending}
      onClose={onClose}
    >
      <form className="session-form" onSubmit={submit} noValidate>
        <fieldset disabled={pending}>
          <legend className="sr-only">Умови заняття</legend>
          {field("title", "Тема заняття", "text", 255)}
          <p className="profile-hint">
            Викладач: {fullName(lesson.teacher)} · {lesson.skill.name} · планова тривалість{" "}
            {lesson.plannedDurationMinutes} хв.
          </p>
          <label className="session-field" htmlFor={`${id}-description`}>
            Опис (необов’язково)
            <textarea
              id={`${id}-description`}
              name="description"
              rows={2}
              maxLength={1000}
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
            />
            {errors.description && <span role="alert">{errors.description}</span>}
          </label>
          <p className="profile-hint">
            Часовий пояс: {Intl.DateTimeFormat().resolvedOptions().timeZone}. Тривалість: 15–240 хв.
          </p>
          <div className="session-time-fields">
            {field("startsAt", "Початок", "datetime-local")}
            {field("endsAt", "Завершення", "datetime-local")}
          </div>
          <label className="session-field" htmlFor={`${id}-format`}>
            Формат
            <select
              id={`${id}-format`}
              value={draft.format}
              onChange={(event) =>
                setDraft({ ...draft, format: event.target.value as SessionDraft["format"] })
              }
            >
              <option value="ONLINE">Онлайн</option>
              <option value="OFFLINE">Офлайн</option>
            </select>
          </label>
          {draft.format === "ONLINE"
            ? field("meetingUrl", "Посилання на зустріч", "url", 500)
            : field("meetingPlace", "Місце зустрічі", "text", 180)}
        </fieldset>
        {failure != null && (
          <p className="exchange-error" role="alert">
            {sessionErrorMessage(failure)}{" "}
            {failure instanceof AuthError && failure.status === 401 && (
              <Link href={loginHref(sessionHref(lesson.id))}>Увійти</Link>
            )}
          </p>
        )}
        <div className="dashboard-modal__actions">
          <button className="dashboard-button" type="button" disabled={pending} onClick={onClose}>
            Назад
          </button>
          <button
            className="dashboard-button dashboard-button--primary"
            type="submit"
            disabled={pending}
          >
            {pending ? "Надсилаємо…" : "Надіслати пропозицію"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
