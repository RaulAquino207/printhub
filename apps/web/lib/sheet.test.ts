import { describe, expect, it } from "vitest"

import { fitOnSheet, sheetAspect, sheetPoints } from "@/lib/sheet"

describe("sheetPoints", () => {
  it("returns A4 in points, swapped for landscape", () => {
    const portrait = sheetPoints("A4", "portrait")
    expect(portrait.width).toBeCloseTo(595.28, 1)
    expect(portrait.height).toBeCloseTo(841.89, 1)
    expect(sheetPoints("A4", "landscape")).toEqual({
      width: portrait.height,
      height: portrait.width,
    })
  })

  it("falls back to A4 for unknown sizes", () => {
    expect(sheetPoints("Tabloid", "portrait")).toEqual(
      sheetPoints("A4", "portrait")
    )
  })

  it("gives Letter a different aspect than A4", () => {
    expect(sheetAspect("Letter", "portrait")).not.toBeCloseTo(
      sheetAspect("A4", "portrait"),
      3
    )
  })
})

describe("fitOnSheet", () => {
  it("keeps content that matches the sheet orientation and scales it to fit", () => {
    expect(fitOnSheet(100, 200, 50, 300)).toEqual({ rotate: false, scale: 0.5 })
  })

  it("turns landscape content on a portrait sheet", () => {
    const p = fitOnSheet(400, 200, 100, 300)
    expect(p.rotate).toBe(true)
    // After turning, the content is 200 wide and 400 tall: limited by width.
    expect(p.scale).toBe(0.5)
  })

  it("turns portrait content on a landscape sheet", () => {
    expect(fitOnSheet(210, 297, 297, 210).rotate).toBe(true)
  })

  it("never turns square content", () => {
    expect(fitOnSheet(100, 100, 100, 300).rotate).toBe(false)
  })
})
