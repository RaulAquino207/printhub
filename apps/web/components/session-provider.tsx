"use client"

import { createContext, use, useCallback, useEffect, useState } from "react"
import { usePathname, useRouter } from "next/navigation"
import { CircleAlert } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { UNAUTHENTICATED_EVENT } from "@/lib/api"
import { fetchMe, logout, type User } from "@/lib/auth"
import { ApiError } from "@/lib/jobs"

export const SIGN_IN_PATH = "/entrar"

type Session =
  | { status: "loading" }
  | { status: "authenticated"; user: User }
  | { status: "anonymous" }
  | { status: "error" } // the API could not be reached

type SessionContextValue = Session & {
  signIn: (user: User) => void
  signOut: () => Promise<void>
  /** Checks the session again, e.g. after the API was unreachable. */
  retry: () => void
}

const SessionContext = createContext<SessionContextValue | null>(null)

export function useSession() {
  const ctx = use(SessionContext)
  if (!ctx) throw new Error("useSession must be used inside SessionProvider")
  return ctx
}

/**
 * Knows who is signed in and keeps everyone else on the sign-in page. The
 * session cookie is HttpOnly and belongs to the API's host, so the web asks
 * the API (GET /auth/me) instead of reading it.
 *
 * This guard is for navigation only; the API enforces access on every request.
 */
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const [session, setSession] = useState<Session>({ status: "loading" })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    fetchMe()
      .then((user) => active && setSession({ status: "authenticated", user }))
      .catch((err) => {
        if (!active) return
        setSession(
          err instanceof ApiError && err.status === 401
            ? { status: "anonymous" }
            : { status: "error" }
        )
      })
    return () => {
      active = false
    }
  }, [attempt])

  // Any API call answering 401 (e.g. an expired session) signs out here too.
  useEffect(() => {
    const onUnauthenticated = () => setSession({ status: "anonymous" })
    window.addEventListener(UNAUTHENTICATED_EVENT, onUnauthenticated)
    return () =>
      window.removeEventListener(UNAUTHENTICATED_EVENT, onUnauthenticated)
  }, [])

  const onSignInPage = pathname === SIGN_IN_PATH

  useEffect(() => {
    if (session.status === "anonymous" && !onSignInPage) {
      router.replace(`${SIGN_IN_PATH}?next=${encodeURIComponent(pathname)}`)
    }
  }, [session.status, onSignInPage, pathname, router])

  const signIn = useCallback(
    (user: User) => setSession({ status: "authenticated", user }),
    []
  )
  const signOut = useCallback(async () => {
    await logout()
    setSession({ status: "anonymous" })
  }, [])
  const retry = useCallback(() => {
    setSession({ status: "loading" })
    setAttempt((n) => n + 1)
  }, [])

  return (
    <SessionContext value={{ ...session, signIn, signOut, retry }}>
      {children}
    </SessionContext>
  )
}

/**
 * Shows its children only to signed-in users (and always on the sign-in
 * page); otherwise a loading placeholder or a connection error.
 */
export function SessionGate({ children }: { children: React.ReactNode }) {
  const session = useSession()
  const onSignInPage = usePathname() === SIGN_IN_PATH

  let content: React.ReactNode
  if (onSignInPage || session.status === "authenticated") {
    content = children
  } else if (session.status === "error") {
    content = (
      <Alert variant="destructive" className="max-w-2xl">
        <CircleAlert aria-hidden />
        <AlertTitle>Não foi possível verificar a sessão.</AlertTitle>
        <AlertDescription>
          <p>Verifique se a API está em execução e tente novamente.</p>
          <Button
            variant="outline"
            size="sm"
            className="text-foreground"
            onClick={session.retry}
          >
            Tentar novamente
          </Button>
        </AlertDescription>
      </Alert>
    )
  } else {
    content = (
      <div aria-busy aria-label="Carregando" className="flex flex-col gap-3">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
    )
  }

  return content
}
