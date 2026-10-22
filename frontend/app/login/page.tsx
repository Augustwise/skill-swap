import type { Metadata } from "next";
import { AuthBrandPanel } from "../components/AuthBrandPanel";
import { LoginForm } from "../components/LoginForm";

export const metadata: Metadata = {
  title: "Вхід — SkillSwap",
  description: "Увійди в SkillSwap, щоб продовжити обмін навичками.",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ mode?: string | string[]; deleted?: string | string[] }>;
}) {
  const { mode, deleted } = await searchParams;
  return (
    <main className="register-page login-page">
      <AuthBrandPanel />
      <section className="register-main" aria-labelledby="login-title">
        <LoginForm
          initialMode={mode === "recovery" ? "recovery" : "login"}
          accountDeleted={deleted === "1"}
        />
      </section>
    </main>
  );
}
