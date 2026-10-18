import type { ReactNode } from "react";
import { AppShell } from "./AppShell";
import type { Profile } from "./OnboardingApi";

export function ProfileShell({
  profile,
  csrfToken,
  children,
  hasUnsavedChanges,
}: {
  profile: Profile;
  csrfToken: string;
  children: ReactNode;
  hasUnsavedChanges: boolean;
}) {
  return (
    <AppShell
      profile={profile}
      csrfToken={csrfToken}
      section="profile"
      title="Мій профіль"
      subtitle="Дані, які бачать студенти у твоєму профілі"
      contentId="profile-content"
      className="profile-shell"
      canLeave={() =>
        !hasUnsavedChanges ||
        window.confirm("Є незбережені зміни. Залишити сторінку без збереження?")
      }
    >
      {children}
    </AppShell>
  );
}
