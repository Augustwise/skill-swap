export type AuthSession = {
  user: { email: string; emailVerified: boolean };
  csrfToken: string;
};

export class AuthError extends Error {
  constructor(
    public status: number,
    public code: string,
    public fields: Record<string, string> = {},
    public retryAfter = 0,
  ) {
    super(authErrorMessage(status, code));
  }
}

function authErrorMessage(status: number, code: string) {
  if (code === "account_suspended") return "Цей акаунт призупинено.";
  if (code === "mail_unavailable") return "Поштовий сервіс недоступний. Спробуй пізніше.";
  if (status === 429) return "Забагато спроб. Зачекай і спробуй ще раз.";
  if (status === 401) return "Сесія завершилася. Увійди в акаунт, щоб продовжити.";
  if (status === 403) return "Не вдалося підтвердити сесію. Онови сторінку.";
  return "Не вдалося виконати запит. Спробуй ще раз.";
}

export async function authRequest<T>(
  path: string,
  options: RequestInit = {},
  csrfToken?: string,
): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body) headers.set("Content-Type", "application/json");
  if (csrfToken) headers.set("X-CSRF-Token", csrfToken);
  const response = await fetch(`/api/v1/${path}`, {
    ...options,
    headers,
    cache: "no-store",
    credentials: "same-origin",
    signal: options.signal
      ? AbortSignal.any([options.signal, AbortSignal.timeout(15000)])
      : AbortSignal.timeout(15000),
  });
  const data = response.status === 204 ? undefined : await response.json().catch(() => null);
  if (!response.ok) {
    const fields = data?.error?.fields;
    throw new AuthError(
      response.status,
      typeof data?.error?.code === "string" ? data.error.code : "request_failed",
      fields && typeof fields === "object"
        ? Object.fromEntries(
            Object.entries(fields).filter(
              (entry): entry is [string, string] => typeof entry[1] === "string",
            ),
          )
        : {},
      Math.max(0, Number(response.headers.get("Retry-After")) || 0),
    );
  }
  if (data === null) throw new Error("Сервер повернув неочікувану відповідь. Спробуй ще раз.");
  return data as T;
}

// A POST with the CSRF token; a stale token is refreshed from /auth/me and the call retried once.
export async function writeRequest<T>(path: string, options: RequestInit, csrfToken: string) {
  try {
    return await authRequest<T>(path, { method: "POST", ...options }, csrfToken);
  } catch (error) {
    if (!(error instanceof AuthError) || error.code !== "csrf_invalid") throw error;
    const session = await authRequest<AuthSession>("auth/me");
    return authRequest<T>(path, { method: "POST", ...options }, session.csrfToken);
  }
}

export function requestError(error: unknown) {
  return error instanceof AuthError
    ? error.message
    : "Не вдалося зв’язатися із сервером. Перевір з’єднання і спробуй ще раз.";
}
