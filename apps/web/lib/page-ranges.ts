export type PageRange = { first: number; last: number }

/**
 * Parses "1-3, 5" into ranges, with the same rules as the API: pages start at
 * 1 and each range is ascending. Returns null when the text is malformed.
 */
export function parsePageRanges(text: string): PageRange[] | null {
  const ranges: PageRange[] = []
  for (const part of text.split(",")) {
    const match = /^\s*(\d+)\s*(?:-\s*(\d+)\s*)?$/.exec(part)
    if (!match) return null
    const first = Number(match[1])
    const last = match[2] === undefined ? first : Number(match[2])
    if (first < 1 || last < first) return null
    ranges.push({ first, last })
  }
  return ranges
}

/**
 * The pages that will be printed, or null for "all pages". Pages beyond the
 * document's end are ignored here; use exceedsPageCount to report them.
 */
export function selectedPages(
  text: string,
  pageCount: number
): Set<number> | null {
  if (text.trim() === "") return null
  const ranges = parsePageRanges(text)
  if (!ranges) return null
  const pages = new Set<number>()
  for (const { first, last } of ranges) {
    for (let p = first; p <= Math.min(last, pageCount); p++) pages.add(p)
  }
  return pages
}

/** Whether any range asks for a page the document does not have. */
export function exceedsPageCount(text: string, pageCount: number) {
  const ranges = text.trim() === "" ? [] : parsePageRanges(text)
  return Boolean(ranges?.some((r) => r.last > pageCount))
}
