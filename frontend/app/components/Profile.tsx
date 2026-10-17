"use client";

import Link from "next/link";
import { useState } from "react";
import { Logo } from "./Logo";
import { ProfileShell } from "./ProfileShell";
import { ProfileSummary } from "./ProfileSummary";
import { ProfileEditor } from "./ProfileEditor";
import { ProfileSkills } from "./ProfileSkills";
import { useProfile } from "./useProfile";

export function Profile() {
  const [section, setSection] = useState("about");
  const [dirty, setDirty] = useState(false);
  const { profile, session, status, retry, setProfile } = useProfile();
  if (status !== "ready" || !profile || !session) {
    return (
      <main className="dashboard-message-page">
        <div className="dashboard-message" role={status === "loading" ? "status" : "alert"}>
          <Logo variant="app" />
          <h1>
            {status === "loading"
              ? "Завантажуємо профіль…"
              : status === "login"
                ? "Увійди, щоб продовжити"
                : "Не вдалося завантажити профіль"}
          </h1>
          <p>
            {status === "loading"
              ? "Готуємо твої збережені дані."
              : "Перевір з’єднання або увійди в акаунт знову."}
          </p>
          {status === "login" && (
            <Link
              className="dashboard-button dashboard-button--primary"
              href="/login?next=/profile"
            >
              Увійти
            </Link>
          )}
          {status === "error" && (
            <button
              className="dashboard-button dashboard-button--primary"
              type="button"
              onClick={retry}
            >
              Спробувати ще раз
            </button>
          )}
        </div>
      </main>
    );
  }
  return (
    <ProfileShell
      key={profile.id}
      profile={profile}
      csrfToken={session.csrfToken}
      hasUnsavedChanges={dirty}
    >
      <nav className="profile-tabs" aria-label="Розділи профілю">
        <button
          type="button"
          aria-current={section === "skills" ? "page" : undefined}
          onClick={() => setSection("skills")}
          aria-controls="profile-skills"
        >
          Мої навички · {profile.teachingSkills.length + profile.learningSkills.length}
        </button>
        <button
          type="button"
          aria-current={section === "about" ? "page" : undefined}
          onClick={() => setSection("about")}
          aria-controls="profile-about"
        >
          Про мене
        </button>
      </nav>
      <div className="profile-columns">
        <div className="profile-sections">
          <div id="profile-skills" hidden={section !== "skills"}>
            <ProfileSkills profile={profile} csrfToken={session.csrfToken} onSaved={setProfile} />
          </div>
          <div id="profile-about" className="profile-sections" hidden={section !== "about"}>
            <ProfileEditor
              profile={profile}
              csrfToken={session.csrfToken}
              onSaved={setProfile}
              onDirtyChange={setDirty}
            />
            <section className="account-card">
              <h2>Пошта університету</h2>
              <p className="profile-email">{session.user.email}</p>
              <span className="profile-verified">✓ Підтверджено</span>
            </section>
          </div>
        </div>
        <ProfileSummary profile={profile} />
      </div>
    </ProfileShell>
  );
}
