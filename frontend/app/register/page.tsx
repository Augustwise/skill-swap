import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { RegistrationForm } from "../components/RegistrationForm";

export const metadata: Metadata = {
  title: "Реєстрація — SkillSwap",
  description: "Створи акаунт SkillSwap та почни обмінюватися навичками зі студентами.",
};

export default function RegisterPage() {
  return (
    <main className="register-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="register-title">
        <RegistrationForm />
      </section>
    </main>
  );
}
