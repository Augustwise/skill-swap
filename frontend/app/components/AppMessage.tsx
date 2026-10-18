import Link from "next/link";
import { loginHref } from "./AppRoutes";
import { Logo } from "./Logo";

export type AppMessageStatus = "loading" | "login" | "error";

// Full-page state shown before the signed-in user's profile is available.
export function AppMessage({
  status,
  subject,
  path,
  onRetry,
}: {
  status: AppMessageStatus;
  subject: string;
  path: string;
  onRetry?: () => void;
}) {
  return (
    <main className="dashboard-message-page">
      <div className="dashboard-message" role={status === "loading" ? "status" : "alert"}>
        <Logo variant="app" />
        <h1>
          {status === "loading"
            ? `Завантажуємо ${subject}…`
            : status === "login"
              ? "Увійди, щоб продовжити"
              : `Не вдалося завантажити ${subject}`}
        </h1>
        <p>
          {status === "loading"
            ? "Готуємо твої дані."
            : status === "login"
              ? "Твоя сесія завершилася. Увійди в акаунт, щоб продовжити."
              : "Перевір з’єднання із сервером і спробуй ще раз."}
        </p>
        {status === "login" && (
          <Link className="dashboard-button dashboard-button--primary" href={loginHref(path)}>
            Увійти
          </Link>
        )}
        {status === "error" && onRetry && (
          <button
            className="dashboard-button dashboard-button--primary"
            type="button"
            onClick={onRetry}
          >
            Спробувати ще раз
          </button>
        )}
      </div>
    </main>
  );
}
