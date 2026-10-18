"use client";

import { useEffect, useRef } from "react";

// Runs load now, then every intervalMs while the tab is visible, and again when it is shown.
// A new key restarts the cycle and aborts the previous request.
export function usePolling(
  load: (signal: AbortSignal) => Promise<unknown>,
  intervalMs: number,
  key: string | number = 0,
) {
  const loader = useRef(load);

  useEffect(() => {
    loader.current = load;
  });

  useEffect(() => {
    const controller = new AbortController();
    const run = () => {
      if (!document.hidden) loader.current(controller.signal).catch(() => {});
    };
    run();
    const timer = window.setInterval(run, intervalMs);
    document.addEventListener("visibilitychange", run);
    return () => {
      controller.abort();
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", run);
    };
  }, [intervalMs, key]);
}
