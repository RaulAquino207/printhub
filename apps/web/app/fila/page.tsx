import type { Metadata } from "next"
import { ListChecks } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"

export const metadata: Metadata = {
  title: "Fila de impressão",
}

export default function PrintQueuePage() {
  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title="Fila de impressão"
        description="Acompanhe os trabalhos enviados às impressoras."
      />
      <EmptyState
        icon={ListChecks}
        title="Nenhum trabalho na fila"
        description="Os trabalhos enviados aparecerão aqui."
      />
    </div>
  )
}
