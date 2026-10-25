import { apiFetch } from "@/lib/api"

// Mirrors the API contract of GET /api/v1/printers.

export type PrinterStatus = "online" | "offline" | "unknown"
export type ColorMode = "monochrome" | "color"
export type Orientation = "portrait" | "landscape"

export type PrinterCapabilities = {
  colorModes: ColorMode[]
  paperSizes: string[]
  orientations: Orientation[]
  duplex: boolean
  pageRanges: boolean
  maxCopies: number
}

export type Printer = {
  id: string
  name: string
  model: string
  status: PrinterStatus
  simulated: boolean
  capabilities: PrinterCapabilities
}

export async function fetchPrinters(signal?: AbortSignal): Promise<Printer[]> {
  const res = await apiFetch("/api/v1/printers", { signal })
  if (!res.ok) {
    throw new Error(`GET /api/v1/printers failed with ${res.status}`)
  }
  const body: { printers: Printer[] } = await res.json()
  return body.printers
}

const paperSizeLabels: Record<string, string> = {
  A4: "A4",
  Letter: "Carta",
  Legal: "Ofício",
}

/** One-line pt-BR summary of what a printer supports. */
export function describeCapabilities(c: PrinterCapabilities): string {
  if (c.colorModes.length === 0) {
    return "Recursos indisponíveis"
  }
  return [
    c.colorModes.includes("color") ? "Colorida" : "Preto e branco",
    c.paperSizes.map((size) => paperSizeLabels[size] ?? size).join(", "),
    c.duplex ? "Frente e verso" : "Só frente",
  ].join(" · ")
}
