"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { AppDialog, closeDialog } from "./AppDialog";
import { loginHref } from "./AppRoutes";
import { useCountersChanged } from "./AppShell";
import { AuthError } from "./AuthApi";
import { fullName } from "./DiscoveryFormat";
import {
  acceptRequest,
  answerRequest,
  availableActions,
  exchangeErrorMessage,
  exchangeHref,
  requestPerspective,
  staleRequest,
  type ExchangeRequest,
  type RequestAction,
} from "./ExchangeApi";

const labels: Record<RequestAction, [string, string]> = {
  accept: ["Прийняти", "Приймаємо…"],
  decline: ["Відхилити", "Відхиляємо…"],
  withdraw: ["Відкликати", "Відкликаємо…"],
};

const done: Record<"decline" | "withdraw", string> = {
  decline: "Запит відхилено.",
  withdraw: "Запит відкликано.",
};

// Errors that mean the request changed elsewhere: the list or page reloads to show it.
function changedElsewhere(error: unknown) {
  return (
    staleRequest(error) ||
    (error instanceof AuthError &&
      ["request_outdated", "request_not_found", "action_not_allowed"].includes(error.code))
  );
}

export function ActionError({ error, path }: { error: unknown; path: string }) {
  return (
    <p className="exchange-error" role="alert">
      {exchangeErrorMessage(error)}
      {error instanceof AuthError && error.status === 401 && (
        <>
          {" "}
          <Link href={loginHref(path)}>Увійти знову</Link>
        </>
      )}
    </p>
  );
}

export function RequestActions({
  request,
  viewerId,
  csrfToken,
  path,
  onChanged,
}: {
  request: ExchangeRequest;
  viewerId: string;
  csrfToken: string;
  path: string;
  onChanged: (notice: string) => void;
}) {
  const router = useRouter();
  const countersChanged = useCountersChanged();
  const [pending, setPending] = useState<RequestAction | null>(null);
  const [confirm, setConfirm] = useState<"decline" | "withdraw" | null>(null);
  const [error, setError] = useState<unknown>(null);
  const busy = useRef(false);
  const actions = availableActions(request, viewerId);
  const partner = fullName(requestPerspective(request, viewerId).partner);
  if (actions.length === 0) return null;

  async function run(action: RequestAction, trigger?: Element | null) {
    if (busy.current) return;
    busy.current = true;
    setPending(action);
    setError(null);
    let leaving = false;
    try {
      if (action === "accept") {
        const { exchange } = await acceptRequest(request.id, csrfToken);
        countersChanged();
        leaving = true;
        router.push(`${exchangeHref(exchange.id)}?started=1`);
        return;
      }
      await answerRequest(request.id, action, csrfToken);
      countersChanged();
      closeDialog(trigger ?? null);
      onChanged(done[action]);
    } catch (failure) {
      if (changedElsewhere(failure)) {
        closeDialog(trigger ?? null);
        countersChanged();
        onChanged(exchangeErrorMessage(failure));
      } else {
        setError(failure);
      }
    } finally {
      // While the exchange page opens, the buttons stay disabled.
      if (!leaving) {
        busy.current = false;
        setPending(null);
      }
    }
  }

  return (
    <>
      {actions.map((action) => (
        <button
          className={`dashboard-button ${action === "accept" ? "dashboard-button--primary" : ""}`}
          type="button"
          key={action}
          disabled={pending !== null}
          aria-haspopup={action === "accept" ? undefined : "dialog"}
          onClick={() => (action === "accept" ? void run(action) : setConfirm(action))}
        >
          {labels[action][pending === action ? 1 : 0]}
        </button>
      ))}
      {error !== null && confirm === null && <ActionError error={error} path={path} />}
      {confirm && (
        <AppDialog
          title={confirm === "decline" ? "Відхилити запит?" : "Відкликати запит?"}
          description={
            confirm === "decline"
              ? `${partner} побачить, що запит відхилено. Рішення не можна скасувати, але згодом можна обмінятися новим запитом.`
              : `${partner} більше не зможе прийняти цей запит. Щоб змінити умови, надішли новий запит.`
          }
          canClose={pending === null}
          onClose={() => {
            setConfirm(null);
            setError(null);
          }}
        >
          {error !== null && <ActionError error={error} path={path} />}
          <div className="dashboard-modal__actions exchange-dialog__actions">
            <button
              className="dashboard-button"
              type="button"
              disabled={pending !== null}
              data-autofocus
              onClick={(event) => closeDialog(event.currentTarget)}
            >
              Скасувати
            </button>
            <button
              className="dashboard-button dashboard-button--danger"
              type="button"
              disabled={pending !== null}
              onClick={(event) => void run(confirm, event.currentTarget)}
            >
              {pending === confirm
                ? labels[confirm][1]
                : confirm === "decline"
                  ? "Відхилити запит"
                  : "Відкликати запит"}
            </button>
          </div>
        </AppDialog>
      )}
    </>
  );
}
