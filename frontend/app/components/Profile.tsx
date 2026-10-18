"use client";

import { useState } from "react";
import { AppMessage } from "./AppMessage";
import { ProfileShell } from "./ProfileShell";
import { AppSkeleton } from "./Skeleton";
import { ProfileSummary } from "./ProfileSummary";
import { ProfileEditor } from "./ProfileEditor";
import { ProfileSkills } from "./ProfileSkills";
import { useProfile } from "./useProfile";

export function Profile() {
  const [section, setSection] = useState("about");
  const [dirty, setDirty] = useState(false);
  const { profile, session, status, retry, setProfile } = useProfile();
  if (status === "loading" || (status === "ready" && (!profile || !session))) {
    return <AppSkeleton page="profile" />;
  }
  if (status !== "ready" || !profile || !session) {
    return (
      <AppMessage
        status={status === "login" ? "login" : "error"}
        subject="профіль"
        path="/profile"
        onRetry={retry}
      />
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
