import { jpegToPdf } from "@/lib/pdf-writer"
import type { Orientation } from "@/lib/printers"
import { sheetPoints } from "@/lib/sheet"

/** Longest side kept: A4 at 300 dpi. Larger photos only add bytes. */
const MAX_SIDE = 3508
const JPEG_QUALITY = 0.9

/**
 * Converts a JPG or PNG into a one-page PDF on the chosen paper, so the API
 * and CUPS keep receiving PDFs only.
 *
 * The image is redrawn on a canvas first, which:
 * - applies the EXIF orientation of phone photos (otherwise they print sideways);
 * - flattens PNG transparency onto white;
 * - reduces huge photos to MAX_SIDE pixels.
 */
export async function imageToPdf(
  file: File,
  paperSize: string,
  orientation: Orientation
): Promise<File> {
  const bitmap = await createImageBitmap(file, {
    imageOrientation: "from-image",
  })
  const scale = Math.min(1, MAX_SIDE / Math.max(bitmap.width, bitmap.height))
  const width = Math.round(bitmap.width * scale)
  const height = Math.round(bitmap.height * scale)

  const canvas = document.createElement("canvas")
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext("2d")!
  ctx.fillStyle = "#ffffff"
  ctx.fillRect(0, 0, width, height)
  ctx.drawImage(bitmap, 0, 0, width, height)
  bitmap.close()

  const blob = await new Promise<Blob>((resolve, reject) =>
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error("could not encode the image"))),
      "image/jpeg",
      JPEG_QUALITY
    )
  )
  const pdf = jpegToPdf(
    new Uint8Array(await blob.arrayBuffer()),
    { width, height },
    sheetPoints(paperSize, orientation)
  )
  // Keep the original name: it is only shown to people, never used as a path.
  return new File([pdf.slice()], file.name, { type: "application/pdf" })
}
