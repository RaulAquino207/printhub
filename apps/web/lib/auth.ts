import { apiFetch } from "@/lib/api"
import { ApiError } from "@/lib/jobs"

export type User = { username: string }

async function toApiError(res: Response) {
  try {
    const body = await res.json()
    return new ApiError(res.status, body.error.code)
  } catch {
    return new ApiError(res.status, "unknown_error")
  }
}

/** The signed-in user; throws ApiError(401) without a valid session. */
export async function fetchMe(): Promise<User> {
  const res = await apiFetch("/api/v1/auth/me")
  if (!res.ok) throw await toApiError(res)
  return (await res.json()).user
}

export async function login(username: string, password: string): Promise<User> {
  const res = await apiFetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) throw await toApiError(res)
  return (await res.json()).user
}

export async function logout() {
  const res = await apiFetch("/api/v1/auth/logout", { method: "POST" })
  if (!res.ok && res.status !== 401) throw await toApiError(res)
}

/**
 * Where to go after signing in. Only same-app paths are accepted, so a link
 * like /entrar?next=https://evil.example cannot redirect elsewhere.
 */
export function safeNext(next: string | null) {
  return next && next.startsWith("/") && !next.startsWith("//") ? next : "/"
}
