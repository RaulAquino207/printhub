"use client"

import { CircleAlert, CircleCheck, CircleHelp, Printer } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { PrinterImage } from "@/components/printer-image"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Skeleton } from "@/components/ui/skeleton"
import type { usePrinters } from "@/hooks/use-printers"
import {
  describeCapabilities,
  type Printer as PrinterData,
  type PrinterStatus,
} from "@/lib/printers"
import { cn } from "@/lib/utils"

const HEADING_ID = "printer-picker-heading"
const ERROR_ID = "printer-picker-error"

/** Lists printers as radio options. Selection is controlled by the parent. */
export function PrinterPicker({
  printers,
  value,
  onValueChange,
  error,
}: {
  printers: ReturnType<typeof usePrinters>
  value?: string
  onValueChange: (id: string) => void
  error?: string
}) {
  return (
    <section aria-labelledby={HEADING_ID} className="flex flex-col gap-3">
      <h2 id={HEADING_ID} className="text-lg font-semibold">
        Impressora
      </h2>

      {printers.status === "loading" && <PrinterListSkeleton />}

      {printers.status === "error" && (
        <Alert variant="destructive">
          <CircleAlert aria-hidden />
          <AlertTitle>Não foi possível carregar as impressoras.</AlertTitle>
          <AlertDescription>
            <p>Verifique se a API está em execução e tente novamente.</p>
            <Button
              variant="outline"
              size="sm"
              className="text-foreground"
              onClick={printers.retry}
            >
              Tentar novamente
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {printers.status === "success" &&
        (printers.printers.length === 0 ? (
          <EmptyState
            icon={Printer}
            title="Nenhuma impressora disponível"
            description="As impressoras aparecerão aqui quando estiverem configuradas."
          />
        ) : (
          <PrinterList
            printers={printers.printers}
            value={value}
            onValueChange={onValueChange}
            invalid={Boolean(error)}
          />
        ))}

      {error && (
        <p id={ERROR_ID} role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </section>
  )
}

function PrinterList({
  printers,
  value,
  onValueChange,
  invalid,
}: {
  printers: PrinterData[]
  value?: string
  onValueChange: (id: string) => void
  invalid: boolean
}) {
  return (
    <RadioGroup
      aria-labelledby={HEADING_ID}
      aria-describedby={invalid ? ERROR_ID : undefined}
      value={value ?? null}
      onValueChange={(id) => onValueChange(id as string)}
      className="gap-0 divide-y rounded-xl border bg-card"
    >
      {printers.map((printer) => {
        const unavailable = printer.status !== "online"

        return (
          <label
            key={printer.id}
            className={cn(
              "flex gap-3 px-4 py-3 first:rounded-t-xl last:rounded-b-xl",
              unavailable
                ? "cursor-not-allowed text-muted-foreground"
                : "cursor-pointer hover:bg-accent"
            )}
          >
            <RadioGroupItem
              value={printer.id}
              disabled={unavailable}
              className="mt-0.5 border-muted-foreground data-disabled:opacity-50"
            />
            <PrinterImage
              printerId={printer.id}
              className={cn(unavailable && "opacity-50")}
            />
            <div className="flex min-w-0 flex-col gap-1">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="font-medium text-foreground">
                  {printer.name}
                </span>
                <PrinterStatusLabel status={printer.status} />
                {printer.simulated && <Badge variant="outline">Simulada</Badge>}
              </div>
              <span className="text-sm text-muted-foreground">
                {printer.model}
              </span>
              <span className="text-sm">
                {describeCapabilities(printer.capabilities)}
              </span>
            </div>
          </label>
        )
      })}
    </RadioGroup>
  )
}

const statusDisplay = {
  online: { label: "Online", icon: CircleCheck, className: "text-success" },
  offline: {
    label: "Offline",
    icon: CircleAlert,
    className: "text-destructive",
  },
  unknown: {
    label: "Status desconhecido",
    icon: CircleHelp,
    className: "text-warning",
  },
} satisfies Record<PrinterStatus, unknown>

function PrinterStatusLabel({ status }: { status: PrinterStatus }) {
  const { label, icon: Icon, className } = statusDisplay[status]

  return (
    <span
      className={cn("flex items-center gap-1 text-xs font-medium", className)}
    >
      <Icon aria-hidden className="size-3.5" />
      {label}
    </span>
  )
}

function PrinterListSkeleton() {
  return (
    <div
      aria-busy
      aria-label="Carregando impressoras"
      className="divide-y rounded-xl border bg-card"
    >
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex gap-3 px-4 py-3">
          <Skeleton className="mt-0.5 size-4 rounded-full" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-3 w-56" />
            <Skeleton className="h-3 w-64" />
          </div>
        </div>
      ))}
    </div>
  )
}
