import { NewPrintJob } from "@/components/new-print-job"
import { PageHeader } from "@/components/page-header"

export default function NewPrintJobPage() {
  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title="Nova impressão"
        description="Envie um PDF, escolha a impressora e as opções de impressão."
      />
      <NewPrintJob />
    </div>
  )
}
