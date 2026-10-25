import Image from "next/image"
import { Printer } from "lucide-react"

import { printerImage } from "@/lib/printer-images"
import { cn } from "@/lib/utils"

const sizes = {
  sm: { box: "h-9 w-12", icon: "size-4", px: 48 },
  md: { box: "h-12 w-16", icon: "size-5", px: 64 },
}

/**
 * The printer's illustration, or a neutral icon when there is none. Always
 * shown next to the printer's name, so it is decorative (empty alt).
 */
export function PrinterImage({
  printerId,
  size = "md",
  className,
}: {
  printerId: string
  size?: keyof typeof sizes
  className?: string
}) {
  const image = printerImage(printerId)
  const s = sizes[size]

  return (
    <span
      aria-hidden
      className={cn(
        // The illustrations have a light background: keep it light in dark
        // mode too, framed by a subtle border.
        "flex shrink-0 items-center justify-center overflow-hidden rounded-md border bg-white",
        s.box,
        !image && "bg-muted",
        className
      )}
    >
      {image ? (
        <Image
          src={image}
          alt=""
          sizes={`${s.px}px`}
          className="h-full w-full object-contain"
        />
      ) : (
        <Printer className={cn(s.icon, "text-muted-foreground")} />
      )}
    </span>
  )
}
