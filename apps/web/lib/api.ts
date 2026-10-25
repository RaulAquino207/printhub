/**
 * Base URL of the PrintHub API. The web and the API are separate origins
 * (ADR 0003), so the browser calls the API cross-origin.
 *
 * NEXT_PUBLIC_ variables are inlined at build time.
 */
export const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

/**
 * Header the API requires on state-changing requests (CSRF protection, ADR
 * 0003). A custom header forces a CORS preflight, which other sites fail.
 */
export const CSRF_HEADER = { "X-Requested-With": "PrintHub" }

/** Fired on window when the API answers 401: the session is gone. */
export const UNAUTHENTICATED_EVENT = "printhub:unauthenticated"

export function notifyUnauthenticated() {
  window.dispatchEvent(new Event(UNAUTHENTICATED_EVENT))
}

/**
 * fetch against the API. Always sends the session cookie
 * (`credentials: "include"`), adds the CSRF header to anything but GET, and
 * announces a 401 so the app can return to the sign-in page.
 */
export async function apiFetch(path: string, init: RequestInit = {}) {
  const method = (init.method ?? "GET").toUpperCase()
  const headers = new Headers(init.headers)
  if (method !== "GET" && method !== "HEAD") {
    for (const [name, value] of Object.entries(CSRF_HEADER)) {
      headers.set(name, value)
    }
  }

  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
    credentials: "include",
  })
  if (res.status === 401 && !path.startsWith("/api/v1/auth/")) {
    notifyUnauthenticated()
  }
  return res
}
