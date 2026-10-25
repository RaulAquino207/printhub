"use client"

import { useState } from "react"
import { CircleAlert } from "lucide-react"

import { DocumentPicker } from "@/components/document-picker"
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
import { fieldMessage, formatBytes } from "@/lib/messages"

type Errors = Record<string, string>

/** Returns errors without the given field. */
function without(errors: Errors, field: string): Errors {
  const rest = { ...errors }
  delete rest[field]
  return rest
}

const PAGE_RANGES = /^\s*\d+(\s*-\s*\d+)?(\s*,\s*\d+(\s*-\s*\d+)?)*\s*$/

function validate(
  file: File | null,
  printerId: string | undefined,
  options: PrintOptions | null,
  maxCopies: number
): Errors {
  const errors: Errors = {}
  if (!file) {
    errors.file = fieldMessage("file", "required")
  } else if (
    file.type !== "application/pdf" &&
    !file.name.toLowerCase().endsWith(".pdf")
  ) {
    errors.file = "Selecione um arquivo PDF."
  } else if (file.size > MAX_UPLOAD_BYTES) {
    errors.file = `O arquivo tem ${formatBytes(file.size)}; o limite é ${formatBytes(MAX_UPLOAD_BYTES)}.`
  }
  if (!printerId) errors.printerId = fieldMessage("printerId", "required")
  if (options) {
    if (!(options.copies >= 1 && options.copies <= maxCopies)) {
      errors.copies = `Escolha entre 1 e ${maxCopies} cópias.`
    }
    if (options.pageRanges && !PAGE_RANGES.test(options.pageRanges)) {
      errors.pageRanges = fieldMessage("pageRanges", "invalid_format")
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

  const available =
    printers.status === "success"
      ? printers.printers.filter((p) => p.status === "online")
      : []
  // Until the user chooses, preselect the first printer that can print now.
  const printerId = chosenPrinterId ?? available[0]?.id
  const printer = available.find((p) => p.id === printerId)
  const effectiveOptions =
    options ?? (printer ? defaultOptions(printer.capabilities) : null)
  const submitting = progress !== null

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
      printer?.capabilities.maxCopies ?? 1
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

    setProgress(0)
    try {
      setJob(
        await createJob(
          { printerId: printer.id, options: effectiveOptions, file },
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
            setErrors((e) => without(e, "file"))
          }}
          error={errors.file}
          disabled={submitting}
        />
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

        {submitting && (
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
          {submitting ? "Enviando…" : "Imprimir"}
        </Button>
      </div>
    </form>
  )
}
