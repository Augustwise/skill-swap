import type { ReactNode } from "react";
import { pageShells } from "./AppNavigation";
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
      title={pageShells.profile.title}
      subtitle={pageShells.profile.subtitle}
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
