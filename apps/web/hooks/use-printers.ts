"use client"

import { useCallback, useEffect, useState } from "react"

import { fetchPrinters, type Printer } from "@/lib/printers"

type State =
  | { status: "loading" }
  | { status: "error" }
  | { status: "success"; printers: Printer[] }

/** Loads the printers from the API; `retry` loads them again. */
export function usePrinters() {
  const [state, setState] = useState<State>({ status: "loading" })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()

    fetchPrinters(controller.signal)
      .then((printers) => setState({ status: "success", printers }))
      .catch(() => {
        if (!controller.signal.aborted) setState({ status: "error" })
      })

    return () => controller.abort()
  }, [attempt])

  const retry = useCallback(() => {
    setState({ status: "loading" })
    setAttempt((n) => n + 1)
  }, [])

  return { ...state, retry }
}
