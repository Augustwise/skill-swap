"use client";

import type { ReactNode } from "react";
import { pageShells, type AppPage, type AppSection } from "./AppNavigation";
import type { AuthSession } from "./AuthApi";
import { AppMessage } from "./AppMessage";
import { AppSkeleton } from "./Skeleton";
import type { Profile } from "./OnboardingApi";
import { useProfile } from "./useProfile";

export type AppContext = { profile: Profile; session: AuthSession };

// Renders the page only for a signed-in user with a verified email and a loaded profile.
export function AppGate({
  page,
  section,
  path,
  children,
}: {
  page: AppPage;
  section?: AppSection;
  path: string;
  children: (context: AppContext) => ReactNode;
}) {
  const { profile, session, status, retry } = useProfile();
  if (status === "loading" || (status === "ready" && (!profile || !session))) {
    return <AppSkeleton page={page} section={section} />;
  }
  if (status !== "ready" || !profile || !session) {
    return (
      <AppMessage
        status={status === "login" ? "login" : "error"}
        subject={pageShells[page].subject}
        path={path}
        onRetry={retry}
      />
    );
  }
  return children({ profile, session });
}
