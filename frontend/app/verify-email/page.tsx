import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { EmailVerification } from "../components/EmailVerification";

export const metadata: Metadata = {
  title: "Підтвердження пошти — SkillSwap",
  description: "Підтвердь університетську пошту, щоб продовжити реєстрацію.",
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const { token } = await searchParams;
  const validToken = typeof token === "string" && /^[A-Za-z0-9_-]{43}$/.test(token) ? token : "";
  return (
    <main className="register-page login-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="verification-title">
        <EmailVerification
          token={validToken}
          invalidToken={token !== undefined && !validToken}
          key={validToken || String(token)}
        />
      </section>
    </main>
  );
}
