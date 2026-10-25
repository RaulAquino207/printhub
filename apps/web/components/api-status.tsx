"use client"

import { useEffect, useState } from "react"
import { CircleAlert, CircleCheck, LoaderCircle } from "lucide-react"

import { apiFetch } from "@/lib/api"
import { cn } from "@/lib/utils"

type Status = "checking" | "online" | "offline"

const POLL_INTERVAL_MS = 10_000

const display = {
  checking: {
    label: "Verificando API",
    icon: LoaderCircle,
    className: "text-muted-foreground",
    spin: true,
  },
  online: {
    label: "API online",
    icon: CircleCheck,
    className: "text-success",
    spin: false,
  },
  offline: {
    label: "API indisponível",
    icon: CircleAlert,
    className: "text-destructive",
    spin: false,
  },
} satisfies Record<Status, unknown>

export function ApiStatus({ className }: { className?: string }) {
  const [status, setStatus] = useState<Status>("checking")

  useEffect(() => {
    const controller = new AbortController()

    async function check() {
      try {
        const res = await apiFetch("/healthz", { signal: controller.signal })
        setStatus(res.ok ? "online" : "offline")
      } catch {
        // A CORS rejection or an unreachable API both end up here.
        if (!controller.signal.aborted) setStatus("offline")
      }
    }

    check()
    const timer = setInterval(check, POLL_INTERVAL_MS)

    return () => {
      controller.abort()
      clearInterval(timer)
    }
  }, [])

  const { label, icon: Icon, className: tone, spin } = display[status]

  return (
    <p
      role="status"
      className={cn(
        "flex items-center gap-1.5 text-xs font-medium",
        tone,
        className
      )}
    >
      <Icon
        aria-hidden
        className={cn("size-4", spin && "motion-safe:animate-spin")}
      />
      {label}
    </p>
  )
}
