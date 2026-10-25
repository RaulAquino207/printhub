"use client"

import { useEffect, useState, type RefObject } from "react"

/**
 * Becomes true once the element is near the viewport, and stays true. Used to
 * render page thumbnails only when they are about to be seen.
 */
export function useInView(
  ref: RefObject<Element | null>,
  rootMargin = "200px"
) {
  const [inView, setInView] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el || inView) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) setInView(true)
      },
      { rootMargin }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref, rootMargin, inView])

  return inView
}
