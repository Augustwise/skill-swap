"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { loginHref } from "./AppRoutes";
import { requestError } from "./AuthApi";
import { discoveryProblem } from "./DiscoveryApi";

export function DiscoveryError({
  error,
  path,
  onRetry,
  resetHref,
  notFound,
}: {
  error: unknown;
  path: string;
  onRetry: () => void;
  resetHref?: string;
  notFound?: ReactNode;
}) {
  const router = useRouter();
  const problem = discoveryProblem(error);

  useEffect(() => {
    if (problem === "unverified") router.replace("/verify-email");
  }, [problem, router]);

  if (problem === "unverified") return null;
  if (problem === "notFound" && notFound) return notFound;
  return (
    <div className="account-card discovery-alert" role="alert">
      {problem === "login" ? (
        <>
          <p>Сесія завершилася. Увійди в акаунт, щоб продовжити.</p>
          <Link className="dashboard-button dashboard-button--primary" href={loginHref(path)}>
            Увійти знову
          </Link>
        </>
      ) : problem === "invalid" && resetHref ? (
        <>
          <p>Деякі параметри пошуку некоректні. Скинь фільтри й спробуй ще раз.</p>
          <Link className="dashboard-button dashboard-button--primary" href={resetHref}>
            Скинути фільтри
          </Link>
        </>
      ) : (
        <>
          <p>{requestError(error)}</p>
          <button
            className="dashboard-button dashboard-button--primary"
            type="button"
            onClick={onRetry}
          >
            Спробувати ще раз
          </button>
        </>
      )}
    </div>
  );
}
