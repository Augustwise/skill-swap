"use client";

import Link from "next/link";
import { useId, useRef, useState, type FormEvent } from "react";
import { AppDialog } from "./AppDialog";
import { loginHref } from "./AppRoutes";
import { AuthError } from "./AuthApi";
import { fullName } from "./DiscoveryFormat";
import {
  cancelSession,
  sessionErrorMessage,
  sessionHref,
  type LessonSession,
  type SessionProposal,
} from "./SessionApi";
import { meetingLink, sessionTime } from "./SessionTime";

const proposalLabels: Record<SessionProposal["status"], string> = {
  PENDING: "Очікує відповіді",
  ACCEPTED: "Погоджено",
  DECLINED: "Відхилено",
  SUPERSEDED: "Замінено новою пропозицією",
};

export function ProposalHistory({ proposals }: { proposals: SessionProposal[] }) {
  return (
    <section className="account-card" aria-labelledby="proposal-history">
      <h2 id="proposal-history">Історія погодження</h2>
      {proposals.length === 0 ? (
        <p className="profile-hint">Пропозицій часу ще немає.</p>
      ) : (
        <ol className="session-history">
          {proposals.map((proposal) => {
            const link = meetingLink(proposal.meetingUrl);
            return (
              <li key={proposal.id}>
                <div className="dashboard-section-heading">
                  <strong>{fullName(proposal.proposedBy)}</strong>
                  <span className="session-status">{proposalLabels[proposal.status]}</span>
                </div>
                <p>
                  <time dateTime={proposal.startsAt}>{sessionTime(proposal.startsAt)}</time> —{" "}
                  <time dateTime={proposal.endsAt}>{sessionTime(proposal.endsAt)}</time>
                </p>
                <p>
                  {proposal.format === "ONLINE" ? "Онлайн" : "Офлайн"}
                  {proposal.meetingPlace && ` · ${proposal.meetingPlace}`}
                </p>
                {link && (
                  <a
                    className="exchange-person__link"
                    href={link}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Посилання на цю зустріч ↗
                  </a>
                )}
                <p className="profile-hint">
                  Запропоновано {sessionTime(proposal.createdAt)}
                  {proposal.respondedBy && ` · Відповідь: ${fullName(proposal.respondedBy)}`}
                  {proposal.respondedAt && `, ${sessionTime(proposal.respondedAt)}`}
                </p>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

export function SessionCancellation({
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
  const [reason, setReason] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [invalid, setInvalid] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy.current) return;
    if (!reason.trim() || Array.from(reason.trim()).length > 500) {
      setInvalid(true);
      event.currentTarget.querySelector("textarea")?.focus();
      return;
    }
    busy.current = true;
    setPending(true);
    setError(null);
    try {
      await cancelSession(lesson.id, reason, csrfToken);
      onSaved();
    } catch (failure) {
      setError(failure);
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  return (
    <AppDialog
      title="Скасувати заняття?"
      description={`«${lesson.title}» буде вилучено з розкладу. Історія збережеться, а прогрес не збільшиться.`}
      canClose={!pending}
      onClose={onClose}
    >
      <form className="session-form" onSubmit={submit} noValidate>
        <div className="session-field">
          <label htmlFor={id}>Причина скасування</label>
          <textarea
            id={id}
            rows={3}
            maxLength={500}
            value={reason}
            disabled={pending}
            data-autofocus
            aria-invalid={invalid}
            aria-describedby={invalid ? `${id}-error` : undefined}
            onChange={(event) => setReason(event.target.value)}
          />
          {invalid && (
            <span id={`${id}-error`} className="exchange-field__error">
              Вкажи причину від 1 до 500 символів.
            </span>
          )}
        </div>
        {error != null && (
          <p className="exchange-error" role="alert">
            {sessionErrorMessage(error)}{" "}
            {error instanceof AuthError && error.status === 401 && (
              <Link href={loginHref(sessionHref(lesson.id))}>Увійти</Link>
            )}
          </p>
        )}
        <div className="dashboard-modal__actions">
          <button className="dashboard-button" type="button" disabled={pending} onClick={onClose}>
            Залишити заняття
          </button>
          <button
            className="dashboard-button dashboard-button--primary"
            type="submit"
            disabled={pending}
          >
            {pending ? "Скасовуємо…" : "Скасувати заняття"}
          </button>
        </div>
      </form>
    </AppDialog>
  );
}
