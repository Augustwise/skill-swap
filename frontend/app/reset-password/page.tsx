import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { ResetLinkNotice, ResetPasswordForm } from "../components/ResetPasswordForm";

export const metadata: Metadata = {
  title: "Новий пароль — SkillSwap",
  description: "Встанови новий пароль для акаунта SkillSwap.",
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const { token } = await searchParams;
  const resetToken = typeof token === "string" && /^[A-Za-z0-9_-]{43}$/.test(token) ? token : "";

  return (
    <main className="register-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="reset-title">
        {resetToken ? (
          <ResetPasswordForm token={resetToken} key={resetToken} />
        ) : (
          <ResetLinkNotice />
        )}
      </section>
    </main>
  );
}
