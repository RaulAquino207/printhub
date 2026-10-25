"use client"

import { useState } from "react"
import { CircleAlert } from "lucide-react"

import { DocumentPicker } from "@/components/document-picker"
import { DocumentPreview } from "@/components/document-preview"
import { JobStatusPanel } from "@/components/job-status"
import {
  defaultOptions,
  PrintOptionsForm,
} from "@/components/print-options-form"
import { PrinterPicker } from "@/components/printer-picker"
import { Alert, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Progress, ProgressLabel } from "@/components/ui/progress"
import { usePrinters } from "@/hooks/use-printers"
import {
  ApiError,
  createJob,
  MAX_UPLOAD_BYTES,
  type Job,
  type PrintOptions,
} from "@/lib/jobs"
import { documentKind } from "@/lib/documents"
import { imageToPdf } from "@/lib/image-to-pdf"
import { fieldMessage, formatBytes } from "@/lib/messages"
import { exceedsPageCount, parsePageRanges } from "@/lib/page-ranges"

type Errors = Record<string, string>

/** Returns errors without the given field. */
function without(errors: Errors, field: string): Errors {
  const rest = { ...errors }
  delete rest[field]
  return rest
}

function validate(
  file: File | null,
  printerId: string | undefined,
  options: PrintOptions | null,
  maxCopies: number,
  pageCount: number | null
): Errors {
  const errors: Errors = {}
  if (!file) {
    errors.file = fieldMessage("file", "required")
  } else if (!documentKind(file)) {
    errors.file = "Selecione um PDF, JPG ou PNG."
  } else if (file.size > MAX_UPLOAD_BYTES) {
    errors.file = `O arquivo tem ${formatBytes(file.size)}; o limite é ${formatBytes(MAX_UPLOAD_BYTES)}.`
  }
  if (!printerId) errors.printerId = fieldMessage("printerId", "required")
  if (options) {
    if (!(options.copies >= 1 && options.copies <= maxCopies)) {
      errors.copies = `Escolha entre 1 e ${maxCopies} cópias.`
    }
    if (options.pageRanges && !parsePageRanges(options.pageRanges)) {
      errors.pageRanges = fieldMessage("pageRanges", "invalid_format")
    } else if (
      pageCount !== null &&
      exceedsPageCount(options.pageRanges, pageCount)
    ) {
      errors.pageRanges =
        pageCount === 1
          ? "O documento tem 1 página."
          : `O documento tem ${pageCount} páginas.`
    }
  }
  return errors
}

/**
 * The "Nova impressão" flow: pick a document, a printer and options, submit,
 * then follow the job. Checks here only give fast feedback; the API validates
 * everything again.
 */
export function NewPrintJob() {
  const printers = usePrinters()
  const [file, setFile] = useState<File | null>(null)
  const [chosenPrinterId, setChosenPrinterId] = useState<string>()
  // null means "the defaults of the selected printer".
  const [options, setOptions] = useState<PrintOptions | null>(null)
  const [errors, setErrors] = useState<Errors>({})
  const [formError, setFormError] = useState<string>()
  const [progress, setProgress] = useState<number | null>(null)
  const [job, setJob] = useState<Job | null>(null)
  // Known once the preview has read the document.
  const [pageCount, setPageCount] = useState<number | null>(null)
  const [converting, setConverting] = useState(false)
  // Changes with every selected file, so the preview starts over.
  const [fileKey, setFileKey] = useState(0)

  const available =
    printers.status === "success"
      ? printers.printers.filter((p) => p.status === "online")
      : []
  // Until the user chooses, preselect the first printer that can print now.
  const printerId = chosenPrinterId ?? available[0]?.id
  const printer = available.find((p) => p.id === printerId)
  const effectiveOptions =
    options ?? (printer ? defaultOptions(printer.capabilities) : null)
  const kind = file ? documentKind(file) : null
  const submitting = progress !== null || converting

  function choosePrinter(id: string) {
    setChosenPrinterId(id)
    setOptions(null) // options depend on the printer's capabilities
    setErrors((e) => without(e, "printerId"))
  }

  function reset() {
    setFile(null)
    setOptions(null)
    setErrors({})
    setFormError(undefined)
    setJob(null)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setFormError(undefined)

    const found = validate(
      file,
      printer?.id,
      effectiveOptions,
      printer?.capabilities.maxCopies ?? 1,
      pageCount
    )
    setErrors(found)
    if (
      Object.keys(found).length > 0 ||
      !file ||
      !printer ||
      !effectiveOptions
    ) {
      return
    }

    try {
      let upload = file
      let options = effectiveOptions
      if (kind === "image") {
        // Images become a one-page PDF on the chosen paper; the API and CUPS
        // only ever receive PDFs.
        setConverting(true)
        upload = await imageToPdf(file, options.paperSize, options.orientation)
        options = { ...options, pageRanges: "" }
        setConverting(false)
      }
      setProgress(0)
      setJob(
        await createJob(
          { printerId: printer.id, options, file: upload },
          setProgress
        )
      )
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        setErrors(
          Object.fromEntries(
            err.fields.map((f) => [f.field, fieldMessage(f.field, f.reason)])
          )
        )
      } else if (err instanceof ApiError && err.code === "file_too_large") {
        setErrors({
          file: `O arquivo excede o limite de ${formatBytes(MAX_UPLOAD_BYTES)}.`,
        })
      } else {
        setFormError(
          "Não foi possível enviar o documento. Verifique a conexão com a API e tente novamente."
        )
      }
    } finally {
      setConverting(false)
      setProgress(null)
    }
  }

  if (job) {
    return <JobStatusPanel initialJob={job} onNewJob={reset} />
  }

  return (
    <form
      onSubmit={submit}
      noValidate
      className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_24rem] lg:items-start"
    >
      <section
        aria-labelledby="document-heading"
        className="flex flex-col gap-3"
      >
        <h2 id="document-heading" className="text-lg font-semibold">
          Documento
        </h2>
        <DocumentPicker
          file={file}
          onChange={(f) => {
            setFile(f)
            setFileKey((k) => k + 1)
            setPageCount(null)
            setErrors((e) => without(without(e, "file"), "pageRanges"))
          }}
          error={errors.file}
          disabled={submitting}
        />
        {file && kind && effectiveOptions && (
          <DocumentPreview
            key={fileKey}
            file={file}
            kind={kind}
            paperSize={effectiveOptions.paperSize}
            orientation={effectiveOptions.orientation}
            colorMode={effectiveOptions.colorMode}
            pageRanges={effectiveOptions.pageRanges}
            onPageCount={setPageCount}
          />
        )}
      </section>

      <div className="flex flex-col gap-8 lg:rounded-xl lg:border lg:bg-card lg:p-6">
        <PrinterPicker
          printers={printers}
          value={printerId}
          onValueChange={choosePrinter}
          error={errors.printerId}
        />

        {printer && effectiveOptions && (
          <section
            aria-labelledby="options-heading"
            className="flex flex-col gap-4"
          >
            <h2 id="options-heading" className="text-lg font-semibold">
              Opções
            </h2>
            <PrintOptionsForm
              capabilities={printer.capabilities}
              allowPageRanges={kind !== "image"}
              value={effectiveOptions}
              onChange={setOptions}
              errors={errors}
              disabled={submitting}
            />
          </section>
        )}

        {formError && (
          <Alert variant="destructive">
            <CircleAlert aria-hidden />
            <AlertTitle>{formError}</AlertTitle>
          </Alert>
        )}

        {progress !== null && (
          <Progress value={Math.round(progress * 100)}>
            <ProgressLabel>Enviando documento…</ProgressLabel>
          </Progress>
        )}

        <Button
          type="submit"
          size="lg"
          className="h-11 w-full"
          disabled={submitting || printers.status !== "success"}
        >
          {converting
            ? "Preparando imagem…"
            : submitting
              ? "Enviando…"
              : "Imprimir"}
        </Button>
      </div>
    </form>
  )
}
