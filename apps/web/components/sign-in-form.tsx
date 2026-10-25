"use client"

import { useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { CircleAlert } from "lucide-react"

import { useSession } from "@/components/session-provider"
import { Alert, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { login, safeNext } from "@/lib/auth"
import { ApiError } from "@/lib/jobs"

export function SignInForm() {
  const session = useSession()
  const router = useRouter()
  const next = safeNext(useSearchParams().get("next"))
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  // Already signed in (or just signed in): leave the sign-in page.
  useEffect(() => {
    if (session.status === "authenticated") router.replace(next)
  }, [session.status, next, router])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError(undefined)
    if (!username || !password) {
      setError("Informe o usuário e a senha.")
      return
    }
    setSubmitting(true)
    try {
      session.signIn(await login(username.trim().toLowerCase(), password))
    } catch (err) {
      setError(
        err instanceof ApiError && err.code === "invalid_credentials"
          ? "Usuário ou senha incorretos."
          : "Não foi possível entrar. Verifique a conexão com a API e tente novamente."
      )
      setPassword("")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form
      onSubmit={submit}
      noValidate
      className="flex flex-col gap-6 rounded-xl border bg-card p-6"
    >
      <FieldGroup className="gap-5">
        <Field>
          <FieldLabel htmlFor="username">Usuário</FieldLabel>
          <Input
            id="username"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={submitting}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="password">Senha</FieldLabel>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={submitting}
          />
        </Field>
      </FieldGroup>

      {error && (
        <Alert variant="destructive">
          <CircleAlert aria-hidden />
          <AlertTitle>{error}</AlertTitle>
        </Alert>
      )}

      <Button
        type="submit"
        size="lg"
        className="h-11 w-full"
        disabled={submitting}
      >
        {submitting ? "Entrando…" : "Entrar"}
      </Button>
    </form>
  )
}
