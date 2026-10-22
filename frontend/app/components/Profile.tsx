"use client";

import { useState } from "react";
import Link from "next/link";
import { AppMessage } from "./AppMessage";
import { ProfileShell } from "./ProfileShell";
import { AppSkeleton } from "./Skeleton";
import { ProfileSummary } from "./ProfileSummary";
import { ProfileEditor } from "./ProfileEditor";
import { ProfileSkills } from "./ProfileSkills";
import { useProfile } from "./useProfile";
import { ReceivedReviews } from "./ReceivedReviews";
import { reviewsHref } from "./ReviewApi";

export function Profile({
  section = "about",
  reviewsPage = 1,
}: {
  section?: "about" | "skills" | "reviews";
  reviewsPage?: number;
}) {
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
        path={reviewsHref(
          section === "about" ? "/profile" : `/profile?tab=${section}`,
          reviewsPage,
        )}
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
        <Link
          href="/profile?tab=skills"
          aria-current={section === "skills" ? "page" : undefined}
          aria-controls="profile-skills"
        >
          Мої навички · {profile.teachingSkills.length + profile.learningSkills.length}
        </Link>
        <Link
          href="/profile"
          aria-current={section === "about" ? "page" : undefined}
          aria-controls="profile-about"
        >
          Про мене
        </Link>
        <Link
          href="/profile?tab=reviews"
          aria-current={section === "reviews" ? "page" : undefined}
          aria-controls="profile-reviews"
        >
          Відгуки про мене
        </Link>
      </nav>
      <div className="profile-columns">
        <div className="profile-sections">
          <div id="profile-skills" hidden={section !== "skills"}>
            <ProfileSkills profile={profile} csrfToken={session.csrfToken} onSaved={setProfile} />
          </div>
          <div id="profile-reviews" hidden={section !== "reviews"}>
            {section === "reviews" && (
              <ReceivedReviews studentId={null} page={reviewsPage} path="/profile?tab=reviews" />
            )}
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
