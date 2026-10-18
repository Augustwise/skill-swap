"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from "react";
import { AppDialog, closeDialog } from "./AppDialog";
import { useRequestsChanged } from "./AppShell";
import { AuthError } from "./AuthApi";
import { DashboardIcon as Icon } from "./DashboardIcon";
import type { StudentProfile } from "./DiscoveryApi";
import { countLabel, formatList, fullName } from "./DiscoveryFormat";
import { ExchangeDirections, PartnerHeading } from "./ExchangeCards";
import {
  MAX_MESSAGE_LENGTH,
  createRequest,
  durationOptions,
  initialProposal,
  proposalErrors,
  proposalTotals,
  requestHref,
  requestsHref,
  sessionOptions,
  validateProposal,
  type ExchangeRequest,
  type ProposalDraft,
  type ProposalErrors,
  type ProposalOptions,
} from "./ExchangeApi";
import { formatDuration, sessionForms } from "./ExchangeFormat";
import type { Profile } from "./OnboardingApi";
import { skillLevels } from "./ProfileApi";
import { ActionError } from "./RequestActions";

function Select({
  id,
  label,
  value,
  error,
  hint,
  disabled,
  initialFocus = false,
  onChange,
  children,
}: {
  id: string;
  label: ReactNode;
  value: string | number;
  error?: string;
  hint?: ReactNode;
  disabled?: boolean;
  initialFocus?: boolean;
  onChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <div className="exchange-field">
      <label className="register-field register-field--select" htmlFor={id}>
        {label}
        <select
          id={id}
          value={value}
          disabled={disabled}
          data-autofocus={initialFocus || undefined}
          aria-invalid={Boolean(error)}
          aria-describedby={error || hint ? `${id}-note` : undefined}
          onChange={(event) => onChange(event.target.value)}
        >
          {children}
        </select>
        <Image src="/figma/register/chevron-down.svg" alt="" width={16} height={16} />
      </label>
      {(error || hint) && (
        <p className={error ? "register-field__error" : "exchange-field__hint"} id={`${id}-note`}>
          {error ?? hint}
        </p>
      )}
    </div>
  );
}

function levelOf(skills: Profile["teachingSkills"], skillId: string) {
  const level = skills.find((skill) => skill.skillId === skillId)?.level;
  return level ? skillLevels[level].toLocaleLowerCase("uk") : "";
}

export function ExchangeProposal({
  profile,
  student,
  options,
  csrfToken,
  onClose,
}: {
  profile: Profile;
  student: StudentProfile;
  options: ProposalOptions;
  csrfToken: string;
  onClose: () => void;
}) {
  const requestsChanged = useRequestsChanged();
  const id = useId();
  const [draft, setDraft] = useState(() => initialProposal(options));
  const [mirrored, setMirrored] = useState(true);
  const [submitted, setSubmitted] = useState(false);
  const [serverErrors, setServerErrors] = useState<ProposalErrors>({});
  const [failure, setFailure] = useState<unknown>(null);
  const [pending, setPending] = useState(false);
  const [created, setCreated] = useState<ExchangeRequest | null>(null);
  const sending = useRef(false);
  const doneLink = useRef<HTMLAnchorElement>(null);
  const name = student.firstName;
  const errors = submitted ? { ...serverErrors, ...validateProposal(draft, options) } : {};
  const totals = proposalTotals(draft);
  const messageLength = Array.from(draft.message).length;
  const teachLevel = options.teach.find((skill) => skill.skillId === draft.teachSkillId);
  const learnLevel = options.learn.find((skill) => skill.skillId === draft.learnSkillId);

  useEffect(() => {
    if (created) doneLink.current?.focus();
  }, [created]);

  function change(patch: Partial<ProposalDraft>) {
    setDraft((current) => {
      const next = { ...current, ...patch };
      return mirrored
        ? {
            ...next,
            learnSessions: next.teachSessions,
            learnDurationMinutes: next.teachDurationMinutes,
          }
        : next;
    });
    setServerErrors({});
    setFailure(null);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending.current) return;
    setSubmitted(true);
    setServerErrors({});
    setFailure(null);
    const invalid = Object.keys(validateProposal(draft, options));
    if (invalid.length > 0) {
      document.getElementById(`${id}-${invalid[0]}`)?.focus();
      return;
    }
    sending.current = true;
    setPending(true);
    try {
      const request = await createRequest(student.id, draft, csrfToken);
      requestsChanged();
      setCreated(request);
    } catch (error) {
      setServerErrors(proposalErrors(error));
      setFailure(error);
    } finally {
      sending.current = false;
      setPending(false);
    }
  }

  if (created) {
    return (
      <AppDialog
        title="Запит надіслано"
        description={`${fullName(student)} побачить його у вхідних і зможе прийняти або відхилити. Поки запит очікує відповіді, його можна відкликати.`}
        className="exchange-proposal"
        onClose={onClose}
      >
        <div className="exchange-proposal__done" role="status">
          <ExchangeDirections
            youTeach={created.requesterTeaches}
            partnerTeaches={created.recipientTeaches}
            partnerName={name}
          />
          <p className="discovery-meta">
            <span>Формат:</span> {formatList([created.format], student.city)} · <span>Разом:</span>{" "}
            {countLabel(created.totalSessions, sessionForms)}
          </p>
        </div>
        <div className="dashboard-modal__actions exchange-dialog__actions">
          <button
            className="dashboard-button"
            type="button"
            onClick={(event) => closeDialog(event.currentTarget)}
          >
            Закрити
          </button>
          <Link
            className="dashboard-button dashboard-button--primary"
            href={requestHref(created.id)}
            ref={doneLink}
          >
            Переглянути запит
          </Link>
        </div>
      </AppDialog>
    );
  }

  const blocked = [
    options.teach.length === 0 && `${name} не хоче вивчати жодної навички з твого списку «Навчаю».`,
    options.learn.length === 0 && `${name} не навчає жодної навички з твого списку «Вивчаю».`,
    options.formats.length === 0 &&
      "У вас немає спільного формату занять (офлайн — лише в одному місті).",
  ].filter((reason): reason is string => Boolean(reason));

  return (
    <AppDialog
      title="Запропонувати обмін"
      description="Обери, чого навчиш і чого хочеш навчитися, формат і план занять. Сумарний час навчання в обидва боки має бути однаковим."
      className="exchange-proposal"
      canClose={!pending}
      onClose={onClose}
    >
      <form className="exchange-proposal__form" noValidate onSubmit={submit}>
        <div className="exchange-proposal__partner">
          <PartnerHeading partner={student} />
        </div>
        {blocked.length > 0 ? (
          <div className="exchange-proposal__blocked" role="status">
            <p>Зараз запит надіслати не вийде:</p>
            <ul>
              {blocked.map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
            <Link className="dashboard-button" href="/profile">
              Оновити мій профіль
            </Link>
          </div>
        ) : (
          <>
            <fieldset className="exchange-proposal__section">
              <legend>Що даю ⇄ що отримую</legend>
              <div className="exchange-proposal__pair">
                <Select
                  id={`${id}-teachSkillId`}
                  label="Ти навчиш"
                  initialFocus
                  value={draft.teachSkillId}
                  error={errors.teachSkillId}
                  hint={
                    teachLevel &&
                    `Твій рівень: ${levelOf(profile.teachingSkills, teachLevel.skillId)} · ${name} зараз: ${skillLevels[teachLevel.level].toLocaleLowerCase("uk")}`
                  }
                  onChange={(teachSkillId) => change({ teachSkillId })}
                >
                  <option value="" disabled>
                    Обери навичку
                  </option>
                  {options.teach.map((skill) => (
                    <option value={skill.skillId} key={skill.skillId}>
                      {skill.name}
                    </option>
                  ))}
                </Select>
                <span className="exchange-proposal__swap" aria-hidden="true">
                  <Icon name="stat-swap" />
                </span>
                <Select
                  id={`${id}-learnSkillId`}
                  label={`${name} навчить тебе`}
                  value={draft.learnSkillId}
                  error={errors.learnSkillId}
                  hint={
                    learnLevel &&
                    `Рівень ${name}: ${skillLevels[learnLevel.level].toLocaleLowerCase("uk")} · твій зараз: ${levelOf(profile.learningSkills, learnLevel.skillId)}`
                  }
                  onChange={(learnSkillId) => change({ learnSkillId })}
                >
                  <option value="" disabled>
                    Обери навичку
                  </option>
                  {options.learn.map((skill) => (
                    <option value={skill.skillId} key={skill.skillId}>
                      {skill.name}
                    </option>
                  ))}
                </Select>
              </div>
            </fieldset>
            <fieldset
              className="exchange-proposal__section"
              aria-describedby={errors.format ? `${id}-format-note` : undefined}
            >
              <legend>Формат занять</legend>
              <div className="exchange-proposal__formats">
                {options.formats.map((format, index) => (
                  <label className="profile-format" key={format}>
                    <input
                      id={index === 0 ? `${id}-format` : undefined}
                      type="radio"
                      name={`${id}-format`}
                      value={format}
                      checked={draft.format === format}
                      onChange={() => change({ format })}
                    />
                    {formatList([format], format === "OFFLINE" ? student.city : "")}
                  </label>
                ))}
              </div>
              {errors.format && (
                <p className="register-field__error" id={`${id}-format-note`}>
                  {errors.format}
                </p>
              )}
            </fieldset>
            <fieldset className="exchange-proposal__section">
              <legend>План занять</legend>
              <label className="exchange-proposal__mirror">
                <input
                  className="checkbox"
                  type="checkbox"
                  checked={mirrored}
                  onChange={(event) => {
                    setMirrored(event.target.checked);
                    if (event.target.checked) {
                      setDraft((current) => ({
                        ...current,
                        learnSessions: current.teachSessions,
                        learnDurationMinutes: current.teachDurationMinutes,
                      }));
                    }
                  }}
                />
                Однакові умови в обидва боки
              </label>
              <div className="exchange-plan">
                {(
                  [
                    ["teach", "Ти проводиш"],
                    ["learn", `${name} проводить`],
                  ] as const
                ).map(([side, title]) => (
                  <div className="exchange-plan__row" key={side}>
                    <p className="exchange-plan__title">{title}</p>
                    <Select
                      id={`${id}-${side === "teach" ? "plan" : "learnSessions"}`}
                      label={
                        <>
                          <span className="sr-only">{title}: </span>Занять
                        </>
                      }
                      value={draft[`${side}Sessions`]}
                      disabled={side === "learn" && mirrored}
                      onChange={(value) =>
                        change(
                          side === "teach"
                            ? { teachSessions: Number(value) }
                            : { learnSessions: Number(value) },
                        )
                      }
                    >
                      {sessionOptions.map((count) => (
                        <option value={count} key={count}>
                          {count}
                        </option>
                      ))}
                    </Select>
                    <Select
                      id={`${id}-${side}Duration`}
                      label={
                        <>
                          <span className="sr-only">{title}: </span>Тривалість
                        </>
                      }
                      value={draft[`${side}DurationMinutes`]}
                      disabled={side === "learn" && mirrored}
                      onChange={(value) =>
                        change(
                          side === "teach"
                            ? { teachDurationMinutes: Number(value) }
                            : { learnDurationMinutes: Number(value) },
                        )
                      }
                    >
                      {durationOptions.map((minutes) => (
                        <option value={minutes} key={minutes}>
                          {formatDuration(minutes)}
                        </option>
                      ))}
                    </Select>
                  </div>
                ))}
              </div>
              <p
                className={`exchange-plan__summary ${errors.plan || totals.teachMinutes !== totals.learnMinutes ? "exchange-plan__summary--error" : ""}`}
                aria-live="polite"
              >
                {totals.teachMinutes === totals.learnMinutes
                  ? `Разом ${countLabel(totals.sessions, sessionForms)}: по ${formatDuration(totals.teachMinutes)} навчання в кожному напрямку.`
                  : `Ти навчаєш ${formatDuration(totals.teachMinutes)}, а ${name} — ${formatDuration(totals.learnMinutes)}. Сумарний час має збігатися.`}
                {errors.plan && totals.teachMinutes === totals.learnMinutes && ` ${errors.plan}`}
              </p>
            </fieldset>
            <div className="exchange-field">
              <label className="exchange-message" htmlFor={`${id}-message`}>
                <span>
                  Повідомлення <span className="exchange-optional">· необов’язково</span>
                </span>
                <textarea
                  id={`${id}-message`}
                  rows={4}
                  value={draft.message}
                  placeholder="Привіт! Розкажи, чому хочеш обмінятися й коли тобі зручно займатися…"
                  aria-invalid={Boolean(errors.message)}
                  aria-describedby={`${id}-message-note`}
                  onChange={(event) =>
                    change({
                      message: Array.from(event.target.value).slice(0, MAX_MESSAGE_LENGTH).join(""),
                    })
                  }
                />
              </label>
              <p
                className={errors.message ? "register-field__error" : "exchange-field__hint"}
                id={`${id}-message-note`}
              >
                {errors.message ?? `${messageLength} / ${MAX_MESSAGE_LENGTH} символів`}
              </p>
            </div>
          </>
        )}
        {failure !== null && (
          <div className="exchange-proposal__failure">
            <ActionError error={failure} path={`/students/${student.id}`} />
            {failure instanceof AuthError && failure.code === "duplicate_request" && (
              <p className="exchange-field__hint">
                Переглянь{" "}
                <Link href={requestsHref({ direction: "outgoing", status: "PENDING" })}>
                  надіслані
                </Link>{" "}
                або <Link href={requestsHref({ status: "PENDING" })}>вхідні</Link> запити.
              </p>
            )}
          </div>
        )}
        <div className="dashboard-modal__actions exchange-dialog__actions">
          <button
            className="dashboard-button"
            type="button"
            disabled={pending}
            onClick={(event) => closeDialog(event.currentTarget)}
          >
            Скасувати
          </button>
          {blocked.length === 0 && (
            <button
              className="dashboard-button dashboard-button--primary"
              type="submit"
              disabled={pending}
            >
              {pending ? "Надсилаємо…" : "Надіслати запит"}
            </button>
          )}
        </div>
      </form>
    </AppDialog>
  );
}
