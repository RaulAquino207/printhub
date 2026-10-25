import type { Metadata } from "next"
import { Geist, Geist_Mono } from "next/font/google"

import "./globals.css"
import { SessionGate, SessionProvider } from "@/components/session-provider"
import { SiteHeader } from "@/components/site-header"
import { ThemeProvider } from "@/components/theme-provider"
import { cn } from "@/lib/utils"

const geist = Geist({
  subsets: ["latin"],
  variable: "--font-sans",
  fallback: ["Inter", "system-ui", "sans-serif"],
})

const fontMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
})

export const metadata: Metadata = {
  title: {
    default: "PrintHub",
    template: "%s · PrintHub",
  },
  description: "Central de impressão privada.",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="pt-BR"
      suppressHydrationWarning
      className={cn(
        "antialiased",
        fontMono.variable,
        "font-sans",
        geist.variable
      )}
    >
      <body>
        <ThemeProvider defaultTheme="light" enableSystem={false}>
          <SessionProvider>
            <SiteHeader />
            <main className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
              <SessionGate>{children}</SessionGate>
            </main>
          </SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
