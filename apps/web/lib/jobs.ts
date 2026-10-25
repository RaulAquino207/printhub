import { API_URL, apiFetch } from "@/lib/api"
import type { ColorMode, Orientation } from "@/lib/printers"

// Mirrors the API contract of /api/v1/jobs.

export type JobStatus =
  | "queued"
  | "processing"
  | "submitted"
  | "printing"
  | "completed"
  | "failed"
  | "cancelled"

export type PrintOptions = {
  copies: number
  colorMode: ColorMode
  paperSize: string
  orientation: Orientation
  duplex: boolean
  pageRanges: string
}

export type Job = {
  id: string
  status: JobStatus
  printer: { id: string; name: string; model: string; simulated: boolean }
  document: { name: string; sizeBytes: number }
  options: PrintOptions
  failureReason?: string
  createdAt: string
  updatedAt: string
  history: { status: JobStatus; at: string }[]
}

export type FieldError = { field: string; reason: string }

/** An error answered by the API, or a network failure (status 0). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly fields: FieldError[] = []
  ) {
    super(`API error ${status}: ${code}`)
  }
}

const terminal: JobStatus[] = ["completed", "failed", "cancelled"]

export function isTerminal(status: JobStatus) {
  return terminal.includes(status)
}

export function isCancellable(status: JobStatus) {
  return status === "queued" || status === "submitted" || status === "printing"
}

/** Largest PDF accepted; the API enforces its own limit too. */
export const MAX_UPLOAD_BYTES =
  Number(process.env.NEXT_PUBLIC_MAX_UPLOAD_MB ?? 50) * 1024 * 1024

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json()
    return new ApiError(res.status, body.error.code, body.error.fields)
  } catch {
    return new ApiError(res.status, "unknown_error")
  }
}

/**
 * Uploads the document and creates the job. Uses XMLHttpRequest instead of
 * fetch because fetch does not report upload progress.
 */
export function createJob(
  input: { printerId: string; options: PrintOptions; file: File },
  onProgress?: (fraction: number) => void
): Promise<Job> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append("printerId", input.printerId)
    form.append("options", JSON.stringify(input.options))
    form.append("file", input.file, input.file.name)

    const xhr = new XMLHttpRequest()
    xhr.open("POST", `${API_URL}/api/v1/jobs`)
    xhr.withCredentials = true // same as fetch's credentials: "include"
    xhr.responseType = "json"
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(e.loaded / e.total)
    }
    xhr.onload = () => {
      if (xhr.status === 201) {
        resolve(xhr.response as Job)
        return
      }
      const error = xhr.response?.error
      reject(
        new ApiError(xhr.status, error?.code ?? "unknown_error", error?.fields)
      )
    }
    xhr.onerror = () => reject(new ApiError(0, "network_error"))
    xhr.send(form)
  })
}

export async function fetchJob(id: string, signal?: AbortSignal) {
  const res = await apiFetch(`/api/v1/jobs/${encodeURIComponent(id)}`, {
    signal,
  })
  if (!res.ok) throw await toApiError(res)
  return (await res.json()) as Job
}

export async function cancelJob(id: string) {
  const res = await apiFetch(`/api/v1/jobs/${encodeURIComponent(id)}/cancel`, {
    method: "POST",
  })
  if (!res.ok) throw await toApiError(res)
  return (await res.json()) as Job
}
