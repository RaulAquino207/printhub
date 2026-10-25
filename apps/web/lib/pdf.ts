import type { PDFDocumentLoadingTask, PDFDocumentProxy } from "pdfjs-dist"

type PdfJs = typeof import("pdfjs-dist")

let pdfjsPromise: Promise<PdfJs> | undefined

/**
 * Loads pdf.js on first use, so its ~1 MB worker is only downloaded on the
 * screen that previews documents.
 */
function loadPdfJs(): Promise<PdfJs> {
  pdfjsPromise ??= import("pdfjs-dist").then((pdfjs) => {
    // Parsing happens in a Web Worker, off the main thread.
    pdfjs.GlobalWorkerOptions.workerSrc = new URL(
      "pdfjs-dist/build/pdf.worker.min.mjs",
      import.meta.url
    ).toString()
    return pdfjs
  })
  return pdfjsPromise
}

export type OpenedPdf = { task: PDFDocumentLoadingTask; doc: PDFDocumentProxy }

/** Why a PDF could not be opened, in terms the UI can explain. */
export class PdfOpenError extends Error {
  constructor(readonly reason: "password" | "invalid") {
    super(`could not open PDF: ${reason}`)
  }
}

/**
 * Opens a local PDF file for preview. The caller must call task.destroy()
 * when done, which frees the worker's memory.
 *
 * Untrusted input: XFA forms are disabled and pages are only ever drawn to a
 * <canvas>, never as HTML. pdf.js 6 has no eval-based font path (the source
 * of CVE-2024-4367), so there is nothing else to switch off.
 */
export async function openPdf(file: File): Promise<OpenedPdf> {
  const pdfjs = await loadPdfJs()
  const task = pdfjs.getDocument({
    data: new Uint8Array(await file.arrayBuffer()),
    enableXfa: false,
  })
  try {
    return { task, doc: await task.promise }
  } catch (err) {
    await task.destroy()
    throw new PdfOpenError(
      err instanceof Error && err.name === "PasswordException"
        ? "password"
        : "invalid"
    )
  }
}
