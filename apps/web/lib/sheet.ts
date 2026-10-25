import type { Orientation } from "@/lib/printers"

/** Paper sizes in millimetres, portrait (width < height). */
const paperMm: Record<string, { width: number; height: number }> = {
  A4: { width: 210, height: 297 },
  Letter: { width: 215.9, height: 279.4 },
  Legal: { width: 215.9, height: 355.6 },
}

const POINTS_PER_MM = 72 / 25.4

/**
 * Sheet size in PDF points (1/72 inch) for a paper size and orientation.
 * Unknown sizes fall back to A4.
 */
export function sheetPoints(paperSize: string, orientation: Orientation) {
  const mm = paperMm[paperSize] ?? paperMm.A4
  const width = mm.width * POINTS_PER_MM
  const height = mm.height * POINTS_PER_MM
  return orientation === "landscape"
    ? { width: height, height: width }
    : { width, height }
}

/** Width / height of the sheet. */
export function sheetAspect(paperSize: string, orientation: Orientation) {
  const { width, height } = sheetPoints(paperSize, orientation)
  return width / height
}

export type Placement = {
  /** Whether the content is turned a quarter turn to match the sheet. */
  rotate: boolean
  /** Scale applied to the content (after rotation) to fit the area. */
  scale: number
}

/**
 * How content of a given size is placed in an area: turned a quarter turn
 * when its orientation differs from the area's (as CUPS's pdftopdf does by
 * default), then scaled to fit while keeping its proportions.
 *
 * This approximates what the printer does; drivers and margins vary.
 */
export function fitOnSheet(
  contentWidth: number,
  contentHeight: number,
  areaWidth: number,
  areaHeight: number
): Placement {
  const contentLandscape = contentWidth > contentHeight
  const areaLandscape = areaWidth > areaHeight
  const rotate =
    contentWidth !== contentHeight && contentLandscape !== areaLandscape
  const [w, h] = rotate
    ? [contentHeight, contentWidth]
    : [contentWidth, contentHeight]
  return { rotate, scale: Math.min(areaWidth / w, areaHeight / h) }
}
