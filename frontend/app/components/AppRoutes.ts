const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const pages = new Set(["/main", "/profile", "/onboarding", "/search", "/matches", "/requests"]);
const entityPage = /^\/(?:students|requests|exchanges)\/([^/]+)$/;

export function isUuid(value: string) {
  return uuid.test(value);
}

// Accepts only known in-app pages, so a crafted ?next= or ?from= cannot leave the site.
export function safeAppPath(value: string | null | undefined, fallback: string) {
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) {
    return fallback;
  }
  let url: URL;
  try {
    url = new URL(value, "http://skillswap.local");
  } catch {
    return fallback;
  }
  if (url.origin !== "http://skillswap.local") return fallback;
  const entity = entityPage.exec(url.pathname);
  if (!pages.has(url.pathname) && !(entity && isUuid(entity[1]))) return fallback;
  return `${url.pathname}${url.search}`;
}

export function loginHref(path: string) {
  return `/login?next=${encodeURIComponent(safeAppPath(path, "/main"))}`;
}

export function studentHref(id: string, from?: string) {
  const path = `/students/${encodeURIComponent(id)}`;
  return from ? `${path}?from=${encodeURIComponent(from)}` : path;
}
