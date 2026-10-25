import type { JobStatus } from "@/lib/jobs"

// pt-BR text for the stable codes the API returns.

export const jobStatusLabels: Record<JobStatus, string> = {
  queued: "Na fila",
  processing: "Preparando",
  submitted: "Aguardando a impressora",
  printing: "Imprimindo",
  // The provider finished the job; that does not prove paper came out.
  completed: "Enviado à impressora",
  failed: "Falhou",
  cancelled: "Cancelado",
}

const failureReasons: Record<string, string> = {
  printer_unavailable: "A impressora está indisponível.",
  document_missing: "O documento enviado não foi encontrado.",
  submit_failed: "A impressora recusou o trabalho.",
  interrupted:
    "O PrintHub foi reiniciado durante o envio. Envie o documento novamente.",
  provider_lost_job:
    "O serviço de impressão perdeu o trabalho. Envie o documento novamente.",
  printer_error: "A impressora informou um erro.",
}

export function failureMessage(reason?: string) {
  return (reason && failureReasons[reason]) ?? "A impressão falhou."
}

const fieldMessages: Record<string, Record<string, string>> = {
  file: {
    required: "Selecione um arquivo PDF.",
    not_pdf: "O arquivo não é um PDF válido.",
  },
  printerId: {
    required: "Selecione uma impressora.",
    not_found: "Impressora não encontrada.",
    unavailable: "A impressora está indisponível no momento.",
  },
  copies: { out_of_range: "Quantidade de cópias fora do limite." },
  pageRanges: {
    invalid_format: "Use números e intervalos, como 1-3, 5.",
  },
}

/** Message for an invalid field, by field and reason code. */
export function fieldMessage(field: string, reason: string) {
  return (
    fieldMessages[field]?.[reason] ??
    (reason === "unsupported"
      ? "Opção não suportada por esta impressora."
      : "Valor inválido.")
  )
}

const bytesFormat = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 })

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${bytesFormat.format(bytes / 1024)} KB`
  return `${bytesFormat.format(bytes / (1024 * 1024))} MB`
}

const timeFormat = new Intl.DateTimeFormat("pt-BR", { timeStyle: "medium" })

export function formatTime(iso: string) {
  return timeFormat.format(new Date(iso))
}
