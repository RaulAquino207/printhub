import type { StaticImageData } from "next/image"

import colorLaserJetCP1215 from "@/public/printers/hp-color-laserjet-cp1215.webp"
import laserJetProM1132 from "@/public/printers/hp-laserjet-pro-m1132.webp"

/**
 * Illustration per printer, by printer ID. Presentation only: the API knows
 * nothing about images, and capabilities never depend on them.
 *
 * - hp-m1132 / hp-cp1215: the CUPS queue names on the Raspberry Pi
 *   (docs/runbooks/raspberry-pi-print-bridge.md).
 * - mono-laser / color-laser: the simulated printers that mirror them.
 */
const imagesByPrinterId: Record<string, StaticImageData> = {
  "hp-m1132": laserJetProM1132,
  "mono-laser": laserJetProM1132,
  "hp-cp1215": colorLaserJetCP1215,
  "color-laser": colorLaserJetCP1215,
}

export function printerImage(printerId: string): StaticImageData | undefined {
  return imagesByPrinterId[printerId]
}
