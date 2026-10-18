"use client";

import type { ReactNode } from "react";
import type { AuthSession } from "./AuthApi";
import { AppMessage } from "./AppMessage";
import type { Profile } from "./OnboardingApi";
import { useProfile } from "./useProfile";

export type AppContext = { profile: Profile; session: AuthSession };

// Renders the page only for a signed-in user with a verified email and a loaded profile.
export function AppGate({
  subject,
  path,
  children,
}: {
  subject: string;
  path: string;
  children: (context: AppContext) => ReactNode;
}) {
  const { profile, session, status, retry } = useProfile();
  if (status !== "ready" || !profile || !session) {
    return (
      <AppMessage
        status={status === "ready" ? "loading" : status}
        subject={subject}
        path={path}
        onRetry={retry}
      />
    );
  }
  return children({ profile, session });
}
