import { Printer } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"

export default function NewPrintJobPage() {
  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title="Nova impressão"
        description="Envie um PDF, escolha a impressora e as opções de impressão."
      />
      <EmptyState
        icon={Printer}
        title="Nenhuma impressora disponível"
        description="As impressoras aparecerão aqui quando estiverem configuradas."
      />
    </div>
  )
}
