"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { AppGate, type AppContext } from "./AppGate";
import { AppShell, useCountersChanged } from "./AppShell";
import { AuthError } from "./AuthApi";
import { loginHref } from "./AppRoutes";
import { DiscoveryEmpty } from "./DiscoveryCards";
import { DiscoveryError } from "./DiscoveryError";
import { fullName } from "./DiscoveryFormat";
import { exchangeDetails, exchangeHref } from "./ExchangeApi";
import {
  decideCurrentSession,
  sessionDetails,
  sessionErrorMessage,
  sessionHref,
} from "./SessionApi";
import { ProposalHistory, SessionCancellation } from "./SessionChanges";
import { SessionForm } from "./SessionForm";
import { SessionFacts } from "./SessionFacts";
import { SkeletonDetail } from "./Skeleton";
import { useApiQuery } from "./useApiQuery";

function SessionView({
  sessionId,
  session,
  profile,
  from,
}: AppContext & { sessionId: string; from?: string }) {
  const path = sessionHref(sessionId, from);
  const result = useApiQuery(path, (signal) => sessionDetails(sessionId, signal));
  const lesson = result.state.status === "ready" ? result.state.data.session : null;
  const exchange = useApiQuery(lesson?.exchangeId ?? null, (signal) =>
    exchangeDetails(lesson!.exchangeId, signal),
  );
  const active = exchange.state.status === "ready" && exchange.state.data.status === "ACTIVE";
  const [planning, setPlanning] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<unknown>(null);
  const [notice, setNotice] = useState("");
  const busy = useRef(false);
  const countersChanged = useCountersChanged();

  function changed(message: string) {
    setPlanning(false);
    setCancelling(false);
    setFailure(null);
    setNotice(message);
    countersChanged();
    result.retry();
  }

  async function decide(action: "accept" | "decline") {
    if (busy.current || !lesson) return;
    busy.current = true;
    setPending(true);
    setFailure(null);
    try {
      await decideCurrentSession(lesson, action, session.csrfToken);
      changed(
        action === "accept"
          ? "Час погоджено. Заняття додано до розкладу."
          : "Пропозицію відхилено.",
      );
    } catch (error) {
      setFailure(error);
      if (error instanceof AuthError && [403, 404, 409].includes(error.status)) result.retry();
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  return (
    <AppShell
      profile={profile}
      csrfToken={session.csrfToken}
      title="Заняття"
      subtitle="Тема, час і погодження зустрічі"
    >
      <Link
        className="discovery-back"
        href={from ?? (lesson ? exchangeHref(lesson.exchangeId) : "/schedule")}
      >
        ← {from?.startsWith("/schedule") ? "До розкладу" : from ? "Назад" : "До обміну"}
      </Link>
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {failure != null && (
        <p className="exchange-error" role="alert">
          {sessionErrorMessage(failure)}{" "}
          {failure instanceof AuthError && failure.status === 401 && (
            <Link href={loginHref(path)}>Увійти</Link>
          )}
        </p>
      )}
      {result.state.status === "loading" && <SkeletonDetail />}
      {result.state.status === "error" && (
        <DiscoveryError
          error={result.state.error}
          path={path}
          onRetry={result.retry}
          notFound={
            <DiscoveryEmpty icon="calendar" title="Заняття не знайдено.">
              <p>Перевір посилання та свою участь в обміні.</p>
            </DiscoveryEmpty>
          }
        />
      )}
      {lesson && (
        <section className="account-card session-detail">
          <div className="dashboard-section-heading">
            <h2>{lesson.title}</h2>
            <span className="exchange-tab__count">{lesson.planOrder}</span>
          </div>
          <SessionFacts lesson={lesson} />
          <p className="profile-hint">
            Часовий пояс: {Intl.DateTimeFormat().resolvedOptions().timeZone}
          </p>
          <p className="profile-hint">
            Викладач: {fullName(lesson.teacher)} · Учень: {fullName(lesson.learner)}
          </p>
          {lesson.description && <p className="profile-biography">{lesson.description}</p>}
          {lesson.status === "DRAFT" && active && (
            <button
              className="dashboard-button dashboard-button--primary"
              onClick={() => setPlanning(true)}
            >
              Запланувати заняття
            </button>
          )}
          {lesson.canCancel && (
            <div className="session-actions">
              <button
                className="dashboard-button"
                disabled={pending}
                onClick={() => setPlanning(true)}
              >
                Перенести заняття
              </button>
              <button
                className="dashboard-button"
                disabled={pending}
                onClick={() => setCancelling(true)}
              >
                Скасувати заняття
              </button>
            </div>
          )}
          {lesson.status === "CANCELLED" && lesson.cancellationReason && (
            <p className="profile-biography">Причина скасування: {lesson.cancellationReason}</p>
          )}
          {exchange.state.status === "error" && (
            <DiscoveryError error={exchange.state.error} path={path} onRetry={exchange.retry} />
          )}
          {lesson.status === "PROPOSED" && (
            <div className="session-agreement">
              <p>
                {lesson.canRespondToProposal
                  ? "Партнер пропонує цей час. Перевір умови перед погодженням."
                  : "Очікуємо відповіді партнера на пропозицію."}
              </p>
              <p className="profile-hint">
                Ти: {lesson.scheduleConfirmedByMe ? "погоджено" : "очікує"} · Партнер:{" "}
                {lesson.scheduleConfirmedByPartner ? "погоджено" : "очікує"}
              </p>
              {lesson.canRespondToProposal && (
                <div className="session-actions">
                  <button
                    className="dashboard-button dashboard-button--primary"
                    disabled={pending}
                    onClick={() => void decide("accept")}
                  >
                    {pending ? "Зберігаємо…" : "Погодити час"}
                  </button>
                  <button
                    className="dashboard-button"
                    disabled={pending}
                    onClick={() => void decide("decline")}
                  >
                    Відхилити
                  </button>
                </div>
              )}
            </div>
          )}
          <button className="dashboard-button" disabled={pending} onClick={result.retry}>
            Оновити дані
          </button>
        </section>
      )}
      {result.state.status === "ready" && (
        <ProposalHistory proposals={result.state.data.proposals} />
      )}
      {cancelling && lesson && (
        <SessionCancellation
          lesson={lesson}
          path={path}
          csrfToken={session.csrfToken}
          onClose={() => setCancelling(false)}
          onSaved={() => changed("Заняття скасовано. Історію збережено.")}
        />
      )}
      {planning && lesson && (
        <SessionForm
          lesson={lesson}
          path={path}
          csrfToken={session.csrfToken}
          onClose={() => setPlanning(false)}
          onSaved={() => changed("Пропозицію надіслано. Очікуємо погодження партнера.")}
        />
      )}
    </AppShell>
  );
}

export function SessionOverview({ sessionId, from }: { sessionId: string; from?: string }) {
  return (
    <AppGate page="lesson" path={sessionHref(sessionId, from)}>
      {(context) => <SessionView {...context} sessionId={sessionId} from={from} />}
    </AppGate>
  );
}
