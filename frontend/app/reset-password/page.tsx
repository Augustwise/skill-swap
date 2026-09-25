import type { Metadata } from "next";
import Link from "next/link";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { ResetPasswordForm } from "../components/ResetPasswordForm";

export const metadata: Metadata = {
  title: "Новий пароль — SkillSwap",
  description: "Встанови новий пароль для акаунта SkillSwap.",
};

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const { token } = await searchParams;
  const resetToken = typeof token === "string" ? token : "";

  return (
    <main className="register-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="reset-title">
        {resetToken ? (
          <ResetPasswordForm token={resetToken} />
        ) : (
          <div className="register-form register-success">
            <h1 id="reset-title">Посилання недійсне</h1>
            <p>Запроси нове посилання для відновлення пароля.</p>
            <Link className="register-submit register-success__link" href="/login">
              До входу
            </Link>
          </div>
        )}
      </section>
    </main>
  );
}
