import { describe, expect, it } from "vitest"

import {
  exceedsPageCount,
  parsePageRanges,
  selectedPages,
} from "@/lib/page-ranges"

describe("parsePageRanges", () => {
  it.each([
    ["1", [{ first: 1, last: 1 }]],
    [
      "1-3,5",
      [
        { first: 1, last: 3 },
        { first: 5, last: 5 },
      ],
    ],
    [
      " 2 - 4 , 7 ",
      [
        { first: 2, last: 4 },
        { first: 7, last: 7 },
      ],
    ],
  ])("parses %j", (text, want) => {
    expect(parsePageRanges(text)).toEqual(want)
  })

  // Same rejections as the API (printer.ParsePageRanges).
  it.each(["", "0", "3-1", "1-", "a-b", "1,,2"])("rejects %j", (text) => {
    expect(parsePageRanges(text)).toBeNull()
  })
})

describe("selectedPages", () => {
  it("means all pages when empty", () => {
    expect(selectedPages("  ", 5)).toBeNull()
  })

  it("lists the selected pages within the document", () => {
    expect([...selectedPages("1-2, 4-9", 5)!]).toEqual([1, 2, 4, 5])
  })
})

describe("exceedsPageCount", () => {
  it("reports ranges past the last page", () => {
    expect(exceedsPageCount("1-3", 3)).toBe(false)
    expect(exceedsPageCount("2, 4", 3)).toBe(true)
    expect(exceedsPageCount("", 3)).toBe(false)
    expect(exceedsPageCount("nonsense", 3)).toBe(false)
  })
})
