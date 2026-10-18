"use client";

import { useEffect, useRef, useState } from "react";

export type QueryState<T> =
  { status: "loading" } | { status: "ready"; data: T } | { status: "error"; error: unknown };

// Loads data for a key; a new key aborts the previous request so stale results never win.
export function useApiQuery<T>(key: string | null, load: (signal: AbortSignal) => Promise<T>) {
  const [state, setState] = useState<{ key: string | null; value: QueryState<T> }>({
    key,
    value: { status: "loading" },
  });
  const [attempt, setAttempt] = useState(0);
  const loader = useRef(load);

  useEffect(() => {
    loader.current = load;
  });

  useEffect(() => {
    if (key === null) return;
    const controller = new AbortController();
    loader
      .current(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) setState({ key, value: { status: "ready", data } });
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) setState({ key, value: { status: "error", error } });
      });
    return () => controller.abort();
  }, [key, attempt]);

  const value: QueryState<T> = state.key === key ? state.value : { status: "loading" };
  return {
    state: value,
    retry: () => {
      setState({ key, value: { status: "loading" } });
      setAttempt((current) => current + 1);
    },
  };
}
