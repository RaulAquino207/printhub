"use client"

import { useSyncExternalStore } from "react"

function subscribe(onChange: () => void) {
  document.addEventListener("visibilitychange", onChange)
  return () => document.removeEventListener("visibilitychange", onChange)
}

/**
 * Whether the tab is visible. Polling hooks pause while it is hidden, so a
 * forgotten tab does not keep calling the API.
 */
export function usePageVisible() {
  return useSyncExternalStore(
    subscribe,
    () => document.visibilityState === "visible",
    () => true
  )
}
