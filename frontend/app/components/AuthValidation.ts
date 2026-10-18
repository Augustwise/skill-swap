import { safeAppPath } from "./AppRoutes";

export function normalizeEmail(value: string) {
  return value.trim().toLowerCase();
}

export function emailError(value: string) {
  if (!value) return "Введи університетську пошту.";
  if (value.length > 320 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
    return "Введи коректну адресу пошти.";
  }
  return "";
}

export function passwordError(value: string) {
  if (!value) return "Введи пароль.";
  if (Array.from(value).length < 12) return "Пароль має містити щонайменше 12 символів.";
  if (new TextEncoder().encode(value).length > 72) return "Пароль не може перевищувати 72 байти.";
  return "";
}

export function registrationErrors(fields: Record<string, string>) {
  const translated: Record<string, string> = {};
  for (const [field, message] of Object.entries(fields)) {
    if (field === "email") {
      translated.email = message.includes("not registered")
        ? "Цей університет ще не зареєстрований у SkillSwap."
        : "Використай коректну університетську пошту.";
    } else if (field === "password") {
      translated.password = message.includes("72")
        ? "Пароль не може перевищувати 72 байти."
        : "Пароль має містити щонайменше 12 символів.";
    } else if (field === "firstName" || field === "lastName") {
      translated[field] = "Введи ім’я або прізвище довжиною до 100 символів.";
    }
  }
  return translated;
}

export function focusInvalid(form: HTMLFormElement, errors: Record<string, string>) {
  const field = Array.from(form.elements).find(
    (element) => element instanceof HTMLElement && errors[element.getAttribute("name") ?? ""],
  );
  if (field instanceof HTMLElement) field.focus();
}

export function loginDestination(search: string) {
  return safeAppPath(new URLSearchParams(search).get("next"), "/onboarding");
}
