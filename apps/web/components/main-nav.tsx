"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"

import { cn } from "@/lib/utils"

const items = [
  { href: "/", label: "Nova impressão" },
  { href: "/fila", label: "Fila de impressão" },
]

export function MainNav({ className }: { className?: string }) {
  const pathname = usePathname()

  return (
    <nav aria-label="Principal" className={cn("-mb-px flex gap-6", className)}>
      {items.map((item) => {
        const active = pathname === item.href

        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "inline-flex h-11 items-center border-b-2 text-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50 sm:h-14",
              active
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {item.label}
          </Link>
        )
      })}
    </nav>
  )
}
