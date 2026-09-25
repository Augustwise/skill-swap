import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { LoginForm } from "../components/LoginForm";

export const metadata: Metadata = {
  title: "Вхід — SkillSwap",
  description: "Увійди в SkillSwap, щоб продовжити обмін навичками.",
};

export default function LoginPage() {
  return (
    <main className="register-page login-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="login-title">
        <LoginForm />
      </section>
    </main>
  );
}
