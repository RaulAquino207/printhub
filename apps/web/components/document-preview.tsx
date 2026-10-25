"use client"

import { useEffect, useEffectEvent, useRef, useState } from "react"
import { Info } from "lucide-react"
import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist"

import { Alert, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { useInView } from "@/hooks/use-in-view"
import type { DocumentKind } from "@/lib/documents"
import { selectedPages } from "@/lib/page-ranges"
import { openPdf, PdfOpenError } from "@/lib/pdf"
import type { ColorMode, Orientation } from "@/lib/printers"
import { fitOnSheet, sheetAspect, sheetPoints } from "@/lib/sheet"
import { cn } from "@/lib/utils"

/** Thumbnails beyond this many are not drawn, to spare memory on phones. */
const MAX_THUMBNAILS = 24
/** Longest side of a thumbnail, in CSS pixels. */
const THUMBNAIL_PX = 200

type Sheet = { paperSize: string; orientation: Orientation }

type PreviewProps = Sheet & {
  file: File
  kind: DocumentKind
  colorMode: ColorMode
  pageRanges: string
  /** Reports the page count (null while unknown or when the PDF cannot be read). */
  onPageCount: (count: number | null) => void
}

/**
 * Approximate preview of what will be printed: each page drawn on a sheet of
 * the chosen paper and orientation, in gray for black-and-white printing, with
 * the pages outside the page range dimmed.
 */
export function DocumentPreview(props: PreviewProps) {
  return (
    <section aria-labelledby="preview-heading" className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <h3 id="preview-heading" className="font-medium">
          Pré-visualização aproximada
        </h3>
        <p className="text-sm text-muted-foreground">
          Margens e ajustes finais dependem da impressora.
        </p>
      </div>
      {props.kind === "pdf" ? (
        <PdfPreview {...props} />
      ) : (
        <ImagePreview {...props} />
      )}
    </section>
  )
}

type PdfState =
  | { status: "loading" }
  | { status: "error"; reason: "password" | "invalid" }
  | { status: "ready"; doc: PDFDocumentProxy }

function PdfPreview({
  file,
  paperSize,
  orientation,
  colorMode,
  pageRanges,
  onPageCount,
}: PreviewProps) {
  const [state, setState] = useState<PdfState>({ status: "loading" })
  // An effect event reads the latest callback without re-running the effect.
  const reportPageCount = useEffectEvent(onPageCount)

  useEffect(() => {
    let cancelled = false
    let destroy: (() => Promise<void>) | undefined
    // No reset here: the parent gives each file its own key, so a new file
    // mounts a fresh component that starts in "loading".

    openPdf(file)
      .then(({ task, doc }) => {
        destroy = () => task.destroy()
        if (cancelled) {
          destroy()
          return
        }
        setState({ status: "ready", doc })
        reportPageCount(doc.numPages)
      })
      .catch((err) => {
        if (cancelled) return
        setState({
          status: "error",
          reason: err instanceof PdfOpenError ? err.reason : "invalid",
        })
      })

    return () => {
      cancelled = true
      // Frees the document's memory in the pdf.js worker.
      destroy?.()
    }
  }, [file])

  if (state.status === "loading") {
    return <ThumbnailGridSkeleton />
  }

  if (state.status === "error") {
    return (
      <Alert>
        <Info aria-hidden />
        <AlertTitle>
          {state.reason === "password"
            ? "Este PDF é protegido por senha e não pode ser pré-visualizado. A impressora pode recusá-lo."
            : "Não foi possível pré-visualizar este PDF. Ele pode estar corrompido."}
        </AlertTitle>
      </Alert>
    )
  }

  const total = state.doc.numPages
  const shown = Math.min(total, MAX_THUMBNAILS)
  const selected = selectedPages(pageRanges, total)

  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm">
        {total === 1 ? "1 página" : `${total} páginas`}
        {selected && (
          <span className="text-muted-foreground">
            {" "}
            · {selected.size} selecionada{selected.size === 1 ? "" : "s"}
          </span>
        )}
      </p>
      <ol className="grid grid-cols-3 gap-3 sm:grid-cols-4">
        {Array.from({ length: shown }, (_, i) => i + 1).map((n) => (
          <li key={n}>
            <PdfPageThumbnail
              doc={state.doc}
              pageNumber={n}
              paperSize={paperSize}
              orientation={orientation}
              grayscale={colorMode === "monochrome"}
              excluded={selected !== null && !selected.has(n)}
            />
          </li>
        ))}
      </ol>
      {total > shown && (
        <p className="text-sm text-muted-foreground">
          Mostrando as primeiras {shown} de {total} páginas.
        </p>
      )}
    </div>
  )
}

function PdfPageThumbnail({
  doc,
  pageNumber,
  paperSize,
  orientation,
  grayscale,
  excluded,
}: Sheet & {
  doc: PDFDocumentProxy
  pageNumber: number
  grayscale: boolean
  excluded: boolean
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const visible = useInView(canvasRef)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!visible || !canvas) return
    let cancelled = false
    let task: RenderTask | undefined

    doc.getPage(pageNumber).then((page) => {
      if (cancelled) return
      const sheet = sheetPoints(paperSize, orientation)
      const natural = page.getViewport({ scale: 1 })
      const { rotate } = fitOnSheet(
        natural.width,
        natural.height,
        sheet.width,
        sheet.height
      )
      const rotation = (page.rotate + (rotate ? 90 : 0)) % 360
      const unscaled = page.getViewport({ scale: 1, rotation })
      const scale =
        (THUMBNAIL_PX * window.devicePixelRatio) /
        Math.max(unscaled.width, unscaled.height)
      const viewport = page.getViewport({ scale, rotation })
      canvas.width = Math.floor(viewport.width)
      canvas.height = Math.floor(viewport.height)
      task = page.render({ canvas, viewport })
      task.promise.catch(() => {
        // Cancelled when the options change before it finished; harmless.
      })
    })

    return () => {
      cancelled = true
      task?.cancel()
    }
  }, [visible, doc, pageNumber, paperSize, orientation])

  return (
    <SheetFrame
      paperSize={paperSize}
      orientation={orientation}
      caption={`Página ${pageNumber}`}
      excluded={excluded}
    >
      <canvas
        ref={canvasRef}
        className={cn("block max-h-full max-w-full", grayscale && "grayscale")}
      />
    </SheetFrame>
  )
}

function ImagePreview({
  file,
  paperSize,
  orientation,
  colorMode,
  onPageCount,
}: PreviewProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [failed, setFailed] = useState(false)
  const reportPageCount = useEffectEvent(onPageCount)

  useEffect(() => {
    reportPageCount(1)
    const canvas = canvasRef.current
    if (!canvas) return
    let cancelled = false

    createImageBitmap(file, { imageOrientation: "from-image" })
      .then((bitmap) => {
        if (cancelled) return bitmap.close()
        const sheet = sheetPoints(paperSize, orientation)
        const { rotate } = fitOnSheet(
          bitmap.width,
          bitmap.height,
          sheet.width,
          sheet.height
        )
        const [w, h] = rotate
          ? [bitmap.height, bitmap.width]
          : [bitmap.width, bitmap.height]
        const scale =
          (THUMBNAIL_PX * 2 * window.devicePixelRatio) / Math.max(w, h)
        canvas.width = Math.round(w * scale)
        canvas.height = Math.round(h * scale)
        const ctx = canvas.getContext("2d")!
        ctx.fillStyle = "#ffffff"
        ctx.fillRect(0, 0, canvas.width, canvas.height)
        if (rotate) {
          // Same quarter turn as the PDF the image becomes (counter-clockwise).
          ctx.translate(0, canvas.height)
          ctx.rotate(-Math.PI / 2)
        }
        ctx.drawImage(bitmap, 0, 0, bitmap.width * scale, bitmap.height * scale)
        bitmap.close()
        setFailed(false)
      })
      .catch(() => !cancelled && setFailed(true))

    return () => {
      cancelled = true
    }
  }, [file, paperSize, orientation])

  if (failed) {
    return (
      <Alert>
        <Info aria-hidden />
        <AlertTitle>Não foi possível ler esta imagem.</AlertTitle>
      </Alert>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm text-muted-foreground">
        A imagem será convertida em PDF, ajustada à folha.
      </p>
      <div className="max-w-56">
        <SheetFrame paperSize={paperSize} orientation={orientation}>
          <canvas
            ref={canvasRef}
            className={cn(
              "block max-h-full max-w-full",
              colorMode === "monochrome" && "grayscale"
            )}
          />
        </SheetFrame>
      </div>
    </div>
  )
}

/** A sheet of paper with the right proportions, holding the page centred. */
function SheetFrame({
  paperSize,
  orientation,
  caption,
  excluded = false,
  children,
}: Sheet & {
  caption?: string
  excluded?: boolean
  children: React.ReactNode
}) {
  return (
    <figure className="flex flex-col gap-1.5">
      <div
        style={{ aspectRatio: sheetAspect(paperSize, orientation) }}
        className={cn(
          // The sheet stays white in dark mode: it represents paper.
          "flex items-center justify-center overflow-hidden rounded-sm border bg-white p-[4%] transition-opacity",
          excluded && "opacity-35"
        )}
      >
        {children}
      </div>
      {caption && (
        <figcaption className="text-xs text-muted-foreground">
          {caption}
          {excluded && <span className="sr-only"> (não será impressa)</span>}
        </figcaption>
      )}
    </figure>
  )
}

function ThumbnailGridSkeleton() {
  return (
    <div
      aria-busy
      aria-label="Carregando a pré-visualização"
      className="grid grid-cols-3 gap-3 sm:grid-cols-4"
    >
      {[0, 1, 2].map((i) => (
        <Skeleton key={i} className="aspect-[210/297] w-full" />
      ))}
    </div>
  )
}
