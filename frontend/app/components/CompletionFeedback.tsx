import Link from "next/link";
import { AuthError } from "./AuthApi";
import { loginHref } from "./AppRoutes";
import { completionErrorMessage, type CompletionState } from "./CompletionApi";

export function CompletionFeedback({
  error,
  notice,
  path,
}: {
  error: unknown;
  notice: string;
  path: string;
}) {
  return (
    <>
      {notice && (
        <p className="profile-success" role="status">
          {notice}
        </p>
      )}
      {error != null && (
        <p className="exchange-error" role="alert">
          {completionErrorMessage(error)}{" "}
          {error instanceof AuthError && error.status === 401 && (
            <Link href={loginHref(path)}>Увійти</Link>
          )}
        </p>
      )}
    </>
  );
}

export function CompletionConfirmations({ item }: { item: CompletionState }) {
  return (
    <dl className="completion-confirmations">
      {[
        { label: "Твоє підтвердження", confirmed: item.completionConfirmedByMe },
        { label: "Підтвердження партнера", confirmed: item.completionConfirmedByPartner },
      ].map(({ label, confirmed }) => (
        <div key={label} data-confirmed={Boolean(confirmed)}>
          <dt>{label}</dt>
          <dd>{confirmed ? "Підтверджено ✓" : "Очікує"}</dd>
        </div>
      ))}
    </dl>
  );
}
