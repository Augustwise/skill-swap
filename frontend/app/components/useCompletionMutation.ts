"use client";

import { useRef, useState } from "react";
import { AuthError } from "./AuthApi";

// Keep feedback on the page while its cards reload after a persisted change.
export function useCompletionMutation(refresh: () => void) {
  const busy = useRef(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [notice, setNotice] = useState("");

  async function run<T>(action: () => Promise<T>, message: string | ((result: T) => string)) {
    if (busy.current) return false;
    busy.current = true;
    setPending(true);
    setError(null);
    setNotice("");
    try {
      const result = await action();
      setNotice(typeof message === "function" ? message(result) : message);
      refresh();
      return true;
    } catch (failure) {
      setError(failure);
      if (failure instanceof AuthError && [403, 404, 409].includes(failure.status)) refresh();
      return false;
    } finally {
      busy.current = false;
      setPending(false);
    }
  }

  return { pending, error, notice, run };
}
