"use client"

import { useState } from "react"
import { LogOut } from "lucide-react"

import { useSession } from "@/components/session-provider"
import { Button } from "@/components/ui/button"

export function UserMenu() {
  const session = useSession()
  const [signingOut, setSigningOut] = useState(false)

  if (session.status !== "authenticated") return null

  return (
    <div className="flex items-center gap-2">
      <span className="hidden text-sm text-muted-foreground sm:inline">
        {session.user.username}
      </span>
      <Button
        variant="ghost"
        size="sm"
        disabled={signingOut}
        onClick={async () => {
          setSigningOut(true)
          try {
            await session.signOut()
          } finally {
            setSigningOut(false)
          }
        }}
      >
        <LogOut aria-hidden data-icon="inline-start" />
        Sair
      </Button>
    </div>
  )
}
