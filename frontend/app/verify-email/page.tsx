import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { EmailVerification } from "../components/EmailVerification";

export const metadata: Metadata = {
  title: "Підтвердження пошти — SkillSwap",
  description: "Підтвердь університетську пошту, щоб продовжити реєстрацію.",
};

export default function VerifyEmailPage() {
  return (
    <main className="register-page login-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="verification-title">
        <EmailVerification />
      </section>
    </main>
  );
}
