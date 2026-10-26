"use client"

import { useCallback, useEffect, useState } from "react"

import { usePageVisible } from "@/hooks/use-page-visible"
import { fetchJobs, type Job, type JobStatus } from "@/lib/jobs"

/** Fast while jobs are moving, slow when the queue is idle. */
const ACTIVE_POLL_MS = 2000
const IDLE_POLL_MS = 15_000
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
 * Active jobs and the most recent finished ones, refreshed every two seconds
 * while some job is active and every fifteen otherwise; paused while the tab
 * is hidden.
 * A failed refresh keeps showing the last data; the error state only appears
 * when nothing could be loaded yet.
 */
export function usePrintQueue() {
  const [state, setState] = useState<State>({ status: "loading" })
  const [attempt, setAttempt] = useState(0)
  const visible = usePageVisible()
  const idle = state.status === "success" && state.active.length === 0

  useEffect(() => {
    if (!visible) return
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
    const timer = setInterval(load, idle ? IDLE_POLL_MS : ACTIVE_POLL_MS)
    return () => {
      controller.abort()
      clearInterval(timer)
    }
  }, [attempt, visible, idle])

  /** Reloads now, e.g. after cancelling a job or to retry after an error. */
  const refresh = useCallback(() => setAttempt((n) => n + 1), [])

  return { ...state, refresh }
}
