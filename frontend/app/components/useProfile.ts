"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AuthError, authRequest, type AuthSession } from "./AuthApi";
import type { Profile } from "./OnboardingApi";

export function useProfile() {
  const router = useRouter();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [session, setSession] = useState<AuthSession | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "login" | "error">("loading");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const signal = AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]);
    async function load() {
      try {
        const current = await authRequest<AuthSession>("auth/me", { signal });
        if (signal.aborted) return;
        if (!current.user.emailVerified) {
          router.replace("/verify-email");
          return;
        }
        const data = await authRequest<{ profile: Profile }>("me/profile", { signal });
        if (signal.aborted) return;
        setSession(current);
        setProfile(data.profile);
        setStatus("ready");
      } catch (error) {
        if (!controller.signal.aborted) {
          setStatus(error instanceof AuthError && error.status === 401 ? "login" : "error");
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [attempt, router]);

  return {
    profile,
    session,
    status,
    setProfile,
    retry: () => {
      setStatus("loading");
      setAttempt((value) => value + 1);
    },
  };
}
