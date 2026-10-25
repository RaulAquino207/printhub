import { PageHeader } from "@/components/page-header"
import { PrinterPicker } from "@/components/printer-picker"

export default function NewPrintJobPage() {
  return (
    <div className="flex max-w-2xl flex-col gap-8">
      <PageHeader
        title="Nova impressão"
        description="Envie um PDF, escolha a impressora e as opções de impressão."
      />
      <PrinterPicker />
    </div>
  )
}
