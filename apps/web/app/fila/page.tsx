import type { Metadata } from "next"

import { PageHeader } from "@/components/page-header"
import { PrintQueue } from "@/components/print-queue"

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
      <PrintQueue />
    </div>
  )
}
