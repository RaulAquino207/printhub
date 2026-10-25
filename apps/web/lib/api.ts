/**
 * Base URL of the PrintHub API. The web and the API are separate origins
 * (ADR 0003), so the browser calls the API cross-origin.
 *
 * NEXT_PUBLIC_ variables are inlined at build time.
 */
export const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

/**
 * fetch against the API. Sends cookies (`credentials: "include"`) so session
 * authentication works across origins once it exists.
 */
export function apiFetch(path: string, init?: RequestInit) {
  return fetch(`${API_URL}${path}`, { credentials: "include", ...init })
}
