"use client"

import { useEffect, useState } from "react"
import Link from "next/link"

import { JobStatusLabel } from "@/components/job-status-label"
import { PrinterImage } from "@/components/printer-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { usePageVisible } from "@/hooks/use-page-visible"
import {
  ApiError,
  cancelJob,
  fetchJob,
  isCancellable,
  isTerminal,
  type Job,
  type JobStatus,
} from "@/lib/jobs"
import {
  failureMessage,
  formatBytes,
  formatTime,
  jobStatusLabels,
} from "@/lib/messages"

const POLL_INTERVAL_MS = 1000

/** Follows a job until it finishes, polling the API every second while the tab is visible. */
export function JobStatusPanel({
  initialJob,
  onNewJob,
}: {
  initialJob: Job
  onNewJob: () => void
}) {
  const [job, setJob] = useState(initialJob)
  const [cancelling, setCancelling] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const finished = isTerminal(job.status)
  const visible = usePageVisible()

  useEffect(() => {
    if (finished || !visible) return
    const controller = new AbortController()
    const timer = setInterval(() => {
      fetchJob(job.id, controller.signal)
        .then(setJob)
        .catch(() => {
          // Transient failures are retried on the next tick.
        })
    }, POLL_INTERVAL_MS)
    return () => {
      controller.abort()
      clearInterval(timer)
    }
  }, [job.id, finished, visible])

  async function cancel() {
    setCancelling(true)
    setActionError(undefined)
    try {
      setJob(await cancelJob(job.id))
    } catch (err) {
      setActionError(
        err instanceof ApiError && err.code === "job_not_cancellable"
          ? "Não é mais possível cancelar este trabalho."
          : "Não foi possível cancelar. Tente novamente."
      )
    } finally {
      setCancelling(false)
    }
  }

  return (
    <section
      aria-labelledby="job-status-heading"
      className="flex max-w-2xl flex-col gap-6 rounded-xl border bg-card p-6"
    >
      <div className="flex items-center gap-4">
        <PrinterImage printerId={job.printer.id} />
        <div className="flex min-w-0 flex-col gap-1">
          <h2 id="job-status-heading" className="text-lg font-semibold">
            Trabalho de impressão
          </h2>
          <p className="text-sm text-muted-foreground">
            {job.document.name} · {formatBytes(job.document.sizeBytes)} ·{" "}
            {job.printer.name}
          </p>
        </div>
      </div>

      <div aria-live="polite" className="flex flex-col gap-2">
        <JobStatusLabel status={job.status} size="lg" />
        <StatusDescription job={job} />
      </div>

      {job.printer.simulated && (
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <Badge variant="outline">Simulada</Badge>
          Impressão simulada: nenhuma folha é impressa.
        </p>
      )}

      <ol className="flex flex-col gap-1 border-l pl-4 text-sm">
        {job.history.map((h, i) => (
          <li key={i} className="flex justify-between gap-4">
            <span>{jobStatusLabels[h.status]}</span>
            <time
              dateTime={h.at}
              className="text-muted-foreground tabular-nums"
            >
              {formatTime(h.at)}
            </time>
          </li>
        ))}
      </ol>

      {actionError && (
        <p role="alert" className="text-sm text-destructive">
          {actionError}
        </p>
      )}

      <div className="flex flex-wrap gap-3">
        {finished ? (
          <Button onClick={onNewJob}>Nova impressão</Button>
        ) : (
          isCancellable(job.status) && (
            <Button variant="outline" onClick={cancel} disabled={cancelling}>
              {cancelling ? "Cancelando…" : "Cancelar impressão"}
            </Button>
          )
        )}
        <Button
          variant="ghost"
          nativeButton={false}
          render={<Link href="/fila" />}
        >
          Ver fila
        </Button>
      </div>
    </section>
  )
}

function StatusDescription({ job }: { job: Job }) {
  const text: Partial<Record<JobStatus, string>> = {
    queued: "O trabalho está na fila do PrintHub.",
    processing: "Enviando o documento para a impressora.",
    submitted: "A impressora recebeu o trabalho e vai começar em instantes.",
    printing: "A impressora está processando o trabalho.",
    completed: job.printer.simulated
      ? "A impressora simulada informou que terminou. Nenhuma folha foi impressa."
      : "A impressora informou que terminou. Confira as folhas na bandeja.",
    cancelled: "O trabalho foi cancelado.",
  }
  const message =
    job.status === "failed"
      ? failureMessage(job.failureReason)
      : text[job.status]
  return <p className="text-sm text-muted-foreground">{message}</p>
}
