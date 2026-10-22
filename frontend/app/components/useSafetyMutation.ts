"use client";

import { useRef, useState } from "react";
import { focusInvalid } from "./AuthValidation";
import { safetyFields } from "./SafetyApi";

export function useSafetyMutation() {
  const busy = useRef(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  async function run<T>(action: () => Promise<T>, form?: HTMLFormElement) {
    if (busy.current) return { ok: false as const };
    busy.current = true;
    setPending(true);
    setError(null);
    setFields({});
    try {
      const value = await action();
      return { ok: true as const, value };
    } catch (failure) {
      const invalid = safetyFields(failure);
      setError(failure);
      setFields(invalid);
      if (form) window.requestAnimationFrame(() => focusInvalid(form, invalid));
      return { ok: false as const };
    } finally {
      busy.current = false;
      setPending(false);
    }
  }
  function validate(form: HTMLFormElement, invalid: Record<string, string>) {
    setFields(invalid);
    setError(null);
    if (!Object.keys(invalid).length) return true;
    focusInvalid(form, invalid);
    return false;
  }
  return { pending, error, fields, run, validate };
}
