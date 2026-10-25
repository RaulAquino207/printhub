"use client"

import { useState } from "react"
import Link from "next/link"
import { CircleAlert, History, Inbox } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { JobStatusLabel } from "@/components/job-status-label"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { usePrintQueue } from "@/hooks/use-print-queue"
import { ApiError, cancelJob, isCancellable, type Job } from "@/lib/jobs"
import { failureMessage, formatBytes, formatWhen } from "@/lib/messages"

export function PrintQueue() {
  const queue = usePrintQueue()

  if (queue.status === "loading") {
    return <QueueSkeleton />
  }

  if (queue.status === "error") {
    return (
      <Alert variant="destructive" className="max-w-2xl">
        <CircleAlert aria-hidden />
        <AlertTitle>Não foi possível carregar a fila.</AlertTitle>
        <AlertDescription>
          <p>Verifique se a API está em execução e tente novamente.</p>
          <Button
            variant="outline"
            size="sm"
            className="text-foreground"
            onClick={queue.refresh}
          >
            Tentar novamente
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="flex flex-col gap-10">
      <section aria-labelledby="active-heading" className="flex flex-col gap-3">
        <h2 id="active-heading" className="text-lg font-semibold">
          Em andamento
        </h2>
        {queue.active.length === 0 ? (
          <div className="flex flex-col items-center gap-4">
            <EmptyState
              icon={Inbox}
              title="Nenhum trabalho em andamento"
              description="Os documentos enviados aparecem aqui até terminarem."
            />
            <Button nativeButton={false} render={<Link href="/" />}>
              Nova impressão
            </Button>
          </div>
        ) : (
          <JobList jobs={queue.active} onChange={queue.refresh} />
        )}
      </section>

      <section aria-labelledby="recent-heading" className="flex flex-col gap-3">
        <h2 id="recent-heading" className="text-lg font-semibold">
          Recentes
        </h2>
        {queue.recent.length === 0 ? (
          <EmptyState
            icon={History}
            title="Nenhum trabalho finalizado"
            description="Trabalhos concluídos, com falha ou cancelados aparecem aqui."
          />
        ) : (
          <JobList jobs={queue.recent} onChange={queue.refresh} />
        )}
      </section>
    </div>
  )
}

function JobList({ jobs, onChange }: { jobs: Job[]; onChange: () => void }) {
  return (
    <ul className="divide-y rounded-xl border bg-card">
      {jobs.map((job) => (
        <JobRow key={job.id} job={job} onChange={onChange} />
      ))}
    </ul>
  )
}

function JobRow({ job, onChange }: { job: Job; onChange: () => void }) {
  const [cancelling, setCancelling] = useState(false)
  const [error, setError] = useState<string>()

  async function cancel() {
    setCancelling(true)
    setError(undefined)
    try {
      await cancelJob(job.id)
      onChange()
    } catch (err) {
      setError(
        err instanceof ApiError && err.code === "job_not_cancellable"
          ? "Não é mais possível cancelar."
          : "Não foi possível cancelar."
      )
    } finally {
      setCancelling(false)
    }
  }

  return (
    <li className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:gap-6">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate font-medium">{job.document.name}</span>
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
          {job.printer.name}
          {job.printer.simulated && <Badge variant="outline">Simulada</Badge>}
          <span aria-hidden>·</span>
          <time dateTime={job.createdAt}>{formatWhen(job.createdAt)}</time>
          <span aria-hidden>·</span>
          {formatBytes(job.document.sizeBytes)}
          {job.createdBy && (
            <>
              <span aria-hidden>·</span>
              por {job.createdBy}
            </>
          )}
        </span>
        {job.status === "failed" && (
          <span className="text-sm text-destructive">
            {failureMessage(job.failureReason)}
          </span>
        )}
        {error && (
          <span role="alert" className="text-sm text-destructive">
            {error}
          </span>
        )}
      </div>

      <div className="flex items-center justify-between gap-4 sm:justify-end">
        <JobStatusLabel status={job.status} className="sm:w-52" />
        {isCancellable(job.status) && (
          <Button
            variant="outline"
            size="sm"
            onClick={cancel}
            disabled={cancelling}
            aria-label={`Cancelar impressão de ${job.document.name}`}
          >
            {cancelling ? "Cancelando…" : "Cancelar"}
          </Button>
        )}
      </div>
    </li>
  )
}

function QueueSkeleton() {
  return (
    <div
      aria-busy
      aria-label="Carregando a fila"
      className="flex flex-col gap-3"
    >
      <Skeleton className="h-6 w-40" />
      <div className="divide-y rounded-xl border bg-card">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex flex-col gap-2 px-4 py-3">
            <Skeleton className="h-4 w-56" />
            <Skeleton className="h-3 w-72" />
          </div>
        ))}
      </div>
    </div>
  )
}
