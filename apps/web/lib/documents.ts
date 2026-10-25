export type DocumentKind = "pdf" | "image"

/**
 * Which kind of document a file is, judged by type and extension, or null if
 * it is not supported. This is only for the UI: the API checks the content.
 */
export function documentKind(file: File): DocumentKind | null {
  const name = file.name.toLowerCase()
  if (file.type === "application/pdf" || name.endsWith(".pdf")) return "pdf"
  if (
    file.type === "image/jpeg" ||
    file.type === "image/png" ||
    /\.(jpe?g|png)$/.test(name)
  ) {
    return "image"
  }
  return null
}

export const ACCEPTED_TYPES =
  "application/pdf,.pdf,image/jpeg,.jpg,.jpeg,image/png,.png"
