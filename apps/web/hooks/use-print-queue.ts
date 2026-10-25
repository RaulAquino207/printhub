"use client"

import { useCallback, useEffect, useState } from "react"

import { fetchJobs, type Job, type JobStatus } from "@/lib/jobs"

const POLL_INTERVAL_MS = 2000
const RECENT_LIMIT = 20

const activeStatuses: JobStatus[] = [
  "queued",
  "processing",
  "submitted",
  "printing",
]
const finishedStatuses: JobStatus[] = ["completed", "failed", "cancelled"]

type State =
  | { status: "loading" }
  | { status: "error" }
  | { status: "success"; active: Job[]; recent: Job[] }

/**
 * Active jobs and the most recent finished ones, refreshed every two seconds.
 * A failed refresh keeps showing the last data; the error state only appears
 * when nothing could be loaded yet.
 */
export function usePrintQueue() {
  const [state, setState] = useState<State>({ status: "loading" })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()

    async function load() {
      try {
        const [active, recent] = await Promise.all([
          fetchJobs({ statuses: activeStatuses }, controller.signal),
          fetchJobs(
            { statuses: finishedStatuses, limit: RECENT_LIMIT },
            controller.signal
          ),
        ])
        setState({ status: "success", active, recent })
      } catch {
        if (controller.signal.aborted) return
        setState((prev) =>
          prev.status === "success" ? prev : { status: "error" }
        )
      }
    }

    load()
    const timer = setInterval(load, POLL_INTERVAL_MS)
    return () => {
      controller.abort()
      clearInterval(timer)
    }
  }, [attempt])

  /** Reloads now, e.g. after cancelling a job or to retry after an error. */
  const refresh = useCallback(() => setAttempt((n) => n + 1), [])

  return { ...state, refresh }
}
