import Link from "next/link"

import { ApiStatus } from "@/components/api-status"
import { MainNav } from "@/components/main-nav"
import { UserMenu } from "@/components/user-menu"

export function SiteHeader() {
  return (
    <header className="border-b bg-background">
      {/* Mobile: brand and API status on the first row, navigation on its own
          full-width row. From sm: everything on a single row. */}
      <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 px-4 sm:flex-nowrap sm:px-6">
        <Link
          href="/"
          className="flex h-14 items-center rounded-sm font-semibold tracking-tight outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          PrintHub
        </Link>
        <MainNav className="order-last w-full sm:order-none sm:w-auto" />
        <div className="ml-auto flex items-center gap-4">
          <ApiStatus />
          <UserMenu />
        </div>
      </div>
    </header>
  )
}
