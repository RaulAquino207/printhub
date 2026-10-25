import { describe, expect, it } from "vitest"

import { jpegToPdf } from "@/lib/pdf-writer"
import { sheetPoints } from "@/lib/sheet"

// Not a decodable image: these tests check the PDF structure, not the pixels.
const fakeJpeg = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 1, 2, 3, 0xff, 0xd9])
const a4 = sheetPoints("A4", "portrait")

function latin1(bytes: Uint8Array) {
  return new TextDecoder("latin1").decode(bytes)
}

describe("jpegToPdf", () => {
  const pdf = jpegToPdf(fakeJpeg, { width: 400, height: 300 }, a4)
  const text = latin1(pdf)

  it("starts like a PDF and embeds the JPEG unchanged", () => {
    expect(text.startsWith("%PDF-1.4\n")).toBe(true)
    expect(text).toContain("/Filter /DCTDecode /Length 9")
    expect(text.endsWith("%%EOF\n")).toBe(true)
  })

  it("has a cross-reference table whose offsets point at each object", () => {
    const startxref = Number(/startxref\n(\d+)\n/.exec(text)![1])
    expect(text.slice(startxref, startxref + 4)).toBe("xref")

    const entries = text.slice(startxref).split("\n").slice(3, 8)
    entries.forEach((entry, i) => {
      expect(entry).toHaveLength(19) // 20 bytes with the newline
      const offset = Number(entry.slice(0, 10))
      expect(text.slice(offset, offset + `${i + 1} 0 obj`.length)).toBe(
        `${i + 1} 0 obj`
      )
    })
  })

  it("turns a landscape image on a portrait page", () => {
    expect(text).toMatch(/q 0 [\d.]+ -[\d.]+ 0 [\d.]+ [\d.]+ cm \/Im0 Do Q/)
    const portrait = latin1(
      jpegToPdf(fakeJpeg, { width: 300, height: 400 }, a4)
    )
    expect(portrait).toMatch(/q [\d.]+ 0 0 [\d.]+ [\d.]+ [\d.]+ cm \/Im0 Do Q/)
  })

  it("is readable by pdf.js", async () => {
    const pdfjs = await import("pdfjs-dist/legacy/build/pdf.mjs")
    // In pdf.js 6 the loading task owns the document: destroy it, not the document.
    const task = pdfjs.getDocument({ data: pdf.slice() })
    const doc = await task.promise
    expect(doc.numPages).toBe(1)
    const page = await doc.getPage(1)
    const [, , width, height] = page.view
    expect(width).toBeCloseTo(a4.width, 1)
    expect(height).toBeCloseTo(a4.height, 1)
    await task.destroy()
  })
})
