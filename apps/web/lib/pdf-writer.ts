import { fitOnSheet } from "@/lib/sheet"

/**
 * Builds a one-page PDF showing a JPEG image fitted on a sheet.
 *
 * A PDF is a header, numbered objects, a cross-reference table with the byte
 * offset of each object, and a trailer pointing at the root object. A JPEG
 * can be embedded as is: PDF readers decode it themselves (/DCTDecode).
 *
 * @param jpeg      JPEG bytes (baseline RGB, as produced by <canvas>).
 * @param pixels    The image's width and height in pixels.
 * @param sheet     Page size in points (1/72 inch).
 * @param margin    Blank border in points, so printers do not clip the image.
 */
export function jpegToPdf(
  jpeg: Uint8Array,
  pixels: { width: number; height: number },
  sheet: { width: number; height: number },
  margin = 12
): Uint8Array {
  const area = {
    width: sheet.width - 2 * margin,
    height: sheet.height - 2 * margin,
  }
  const { rotate, scale } = fitOnSheet(
    pixels.width,
    pixels.height,
    area.width,
    area.height
  )

  // Size of the image as drawn, before any rotation.
  const w = pixels.width * scale
  const h = pixels.height * scale
  // Footprint on the page, after rotation, centred.
  const [fw, fh] = rotate ? [h, w] : [w, h]
  const x = (sheet.width - fw) / 2
  const y = (sheet.height - fh) / 2

  // An image is drawn into the unit square; the "cm" matrix [a b c d e f]
  // maps (u, v) to (a·u + c·v + e, b·u + d·v + f). The rotated variant turns
  // the image a quarter turn counter-clockwise.
  const matrix = rotate ? [0, w, -h, 0, x + h, y] : [w, 0, 0, h, x, y]
  const content = `q ${matrix.map(num).join(" ")} cm /Im0 Do Q`

  const enc = new TextEncoder()
  const parts: Uint8Array[] = []
  const offsets: number[] = []
  let length = 0
  const push = (bytes: Uint8Array) => {
    parts.push(bytes)
    length += bytes.length
  }
  const text = (s: string) => push(enc.encode(s))
  const object = (n: number, body: () => void) => {
    offsets[n] = length
    text(`${n} 0 obj\n`)
    body()
    text("\nendobj\n")
  }

  // The second line's bytes above 127 tell tools the file is binary.
  push(
    new Uint8Array([...enc.encode("%PDF-1.4\n%"), 0xe2, 0xe3, 0xcf, 0xd3, 0x0a])
  )
  object(1, () => text("<< /Type /Catalog /Pages 2 0 R >>"))
  object(2, () => text("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"))
  object(3, () =>
    text(
      `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${num(sheet.width)} ${num(sheet.height)}] ` +
        `/Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>`
    )
  )
  object(4, () => {
    text(
      `<< /Type /XObject /Subtype /Image /Width ${pixels.width} /Height ${pixels.height} ` +
        `/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>\nstream\n`
    )
    push(jpeg)
    text("\nendstream")
  })
  object(5, () => {
    const bytes = enc.encode(content)
    text(`<< /Length ${bytes.length} >>\nstream\n`)
    push(bytes)
    text("\nendstream")
  })

  // Each cross-reference entry is exactly 20 bytes: offset, generation, "n".
  const xref = length
  text("xref\n0 6\n0000000000 65535 f \n")
  for (let n = 1; n <= 5; n++) {
    text(`${String(offsets[n]).padStart(10, "0")} 00000 n \n`)
  }
  text(`trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`)

  const out = new Uint8Array(length)
  let at = 0
  for (const part of parts) {
    out.set(part, at)
    at += part.length
  }
  return out
}

function num(n: number) {
  return Number(n.toFixed(2)).toString()
}
