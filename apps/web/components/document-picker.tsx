"use client"

import { useRef, useState } from "react"
import { FileImage, FileText, FileUp, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ACCEPTED_TYPES, documentKind } from "@/lib/documents"
import { MAX_UPLOAD_BYTES } from "@/lib/jobs"
import { formatBytes } from "@/lib/messages"
import { cn } from "@/lib/utils"

const ERROR_ID = "document-picker-error"

/**
 * Selects one PDF, by drag and drop or with the file dialog (the keyboard and
 * touch alternative). Validation happens in the parent.
 */
export function DocumentPicker({
  file,
  onChange,
  error,
  disabled,
}: {
  file: File | null
  onChange: (file: File | null) => void
  error?: string
  disabled?: boolean
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)

  function pick(files: FileList | null) {
    onChange(files?.[0] ?? null)
    // Allow selecting the same file again after removing it.
    if (inputRef.current) inputRef.current.value = ""
  }

  return (
    <div className="flex flex-col gap-2">
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPTED_TYPES}
        className="sr-only"
        tabIndex={-1}
        aria-hidden
        disabled={disabled}
        onChange={(e) => pick(e.target.files)}
      />

      {file ? (
        <div className="flex items-center gap-3 rounded-xl border bg-card px-4 py-3">
          {documentKind(file) === "image" ? (
            <FileImage
              aria-hidden
              className="size-5 shrink-0 text-muted-foreground"
            />
          ) : (
            <FileText
              aria-hidden
              className="size-5 shrink-0 text-muted-foreground"
            />
          )}
          <div className="flex min-w-0 flex-1 flex-col">
            <span className="truncate font-medium">{file.name}</span>
            <span className="text-sm text-muted-foreground">
              {formatBytes(file.size)}
            </span>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={disabled}
            onClick={() => inputRef.current?.click()}
          >
            Trocar
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Remover arquivo"
            disabled={disabled}
            onClick={() => onChange(null)}
          >
            <X aria-hidden />
          </Button>
        </div>
      ) : (
        <div
          onDragOver={(e) => {
            e.preventDefault()
            if (!disabled) setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault()
            setDragging(false)
            if (!disabled) pick(e.dataTransfer.files)
          }}
          className={cn(
            "flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center transition-colors",
            dragging && "border-ring bg-accent",
            error && "border-destructive"
          )}
        >
          <FileUp aria-hidden className="size-6 text-muted-foreground" />
          <div className="flex flex-col gap-1">
            <p className="font-medium">Arraste um documento para cá</p>
            <p className="text-sm text-muted-foreground">
              ou selecione um arquivo · PDF, JPG ou PNG de até{" "}
              {formatBytes(MAX_UPLOAD_BYTES)}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            aria-describedby={error ? ERROR_ID : undefined}
            onClick={() => inputRef.current?.click()}
          >
            Selecionar arquivo
          </Button>
        </div>
      )}

      {error && (
        <p id={ERROR_ID} role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
