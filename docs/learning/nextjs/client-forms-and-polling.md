# Client forms, uploads and polling in Next.js

How the "Nova impressão" screen collects a PDF and print options, uploads with progress, and follows the job until it finishes.

## 1. The problem

The main screen must let someone pick a PDF, choose a printer and only the options that printer supports, submit once, see upload progress, and then watch the job move through its states — on a phone and on a desktop, in pt-BR, accessibly. Everything happens in the browser and talks to an API on another origin.

## 2. The theory

- **Client Components** (`"use client"`) are needed for anything interactive: state (`useState`), effects (`useEffect`), event handlers, browser APIs. The page itself stays a Server Component that renders the client flow.
- **Lifting state up**: when two components depend on the same data (the selected printer decides which options exist), the state lives in their closest common parent, and children become **controlled components** — they receive `value` and report changes through callbacks.
- **Derived state**: values computable from other state should be computed during render, not stored and synchronized with effects. Fewer states, fewer inconsistencies.
- **`fetch` cannot report upload progress**; `XMLHttpRequest` can, through `xhr.upload.onprogress`. `withCredentials = true` is its equivalent of `credentials: "include"`.
- **Polling**: an effect starts an interval and returns a cleanup that stops it; React runs the cleanup when the component unmounts or the dependencies change.
- **Accessible feedback**: `aria-live="polite"` announces status changes to screen readers; `role="alert"` announces errors; labels are associated with inputs (`htmlFor`/`id`); a hidden file input is driven by a visible button, so the keyboard and touch have an alternative to drag and drop.

## 3. The chosen solution

```text
app/page.tsx                    Server Component: page header + <NewPrintJob />
components/new-print-job.tsx    Client: owns file, printer, options, errors, progress, job
├── document-picker.tsx         controlled: drag and drop + "Selecionar arquivo"
├── printer-picker.tsx          controlled: radio list (offline printers disabled)
├── print-options-form.tsx      controlled: only the capabilities of the selected printer
└── job-status.tsx              after submit: polls GET /jobs/{id} every second
lib/jobs.ts                     API contract, createJob (XHR with progress), fetchJob, cancelJob
lib/messages.ts                 pt-BR text for status, failure and validation codes
```

Decisions:

- **Derived defaults instead of synchronization.** `options` state is `null` until the user changes something; the rendered options are `options ?? defaultOptions(printer.capabilities)`. Changing printer resets `options` to `null`. The preselected printer is likewise derived (`chosenPrinterId ?? first online printer`), so no effect is needed to "initialize" anything.
- **Capabilities decide the form.** A monochrome printer shows "Cor: Preto e branco (única opção desta impressora)" instead of a choice; duplex and page ranges appear only when supported; paper sizes come from the printer.
- **Validation twice.** The client checks type, size, copies range and page-range syntax for instant feedback; the API validates everything again and its field errors (`{field, reason}`) are mapped to the same pt-BR messages.
- **One primary action.** "Imprimir" is the only primary button; it is disabled while uploading to prevent double submission, and a progress bar shows the upload.
- **Honest status.** "completed" reads "Enviado à impressora"; for simulated printers the screen says explicitly that no paper was printed.

## 4. Example from our project

Derived state in `new-print-job.tsx`:

```tsx
const printerId = chosenPrinterId ?? available[0]?.id
const printer = available.find((p) => p.id === printerId)
const effectiveOptions =
  options ?? (printer ? defaultOptions(printer.capabilities) : null)
```

Upload with progress in `lib/jobs.ts`:

```ts
const xhr = new XMLHttpRequest()
xhr.open("POST", `${API_URL}/api/v1/jobs`)
xhr.withCredentials = true
xhr.upload.onprogress = (e) => {
  if (e.lengthComputable) onProgress?.(e.loaded / e.total)
}
xhr.send(form) // FormData: printerId, options (JSON), file
```

Polling with cleanup in `job-status.tsx`:

```tsx
useEffect(() => {
  if (finished) return
  const controller = new AbortController()
  const timer = setInterval(() => {
    fetchJob(job.id, controller.signal).then(setJob).catch(() => {})
  }, 1000)
  return () => {
    controller.abort()
    clearInterval(timer)
  }
}, [job.id, finished])
```

When the job reaches a terminal status, `finished` changes, React runs the cleanup, and polling stops.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| Server Actions for the upload | Same-origin by design and convenient, but the upload would pass through the Next.js server before reaching the API, and there is no progress reporting |
| `react-hook-form` + `zod` | Great for large forms; for one screen with capability-driven fields, plain state is easier to follow |
| TanStack Query / SWR for polling | Caching, retries and refetch intervals for free; one more dependency for two requests |
| Server-Sent Events or WebSockets for status | Instant updates, but more API surface; one request per second is cheap here |
| `fetch` with `ReadableStream` request bodies | Streaming uploads need HTTP/2 and still do not report progress portably |

## 6. How to validate

```bash
moon run api:dev web:dev
# http://localhost:3000
```

- Click "Imprimir" with nothing selected: errors appear next to "Documento".
- Choose "Laser colorida": the color choice appears; "Laser P&B" shows black and white only.
- Upload a PDF to "Laser P&B": progress, then "Aguardando a impressora" → "Imprimindo" → "Enviado à impressora" in about 7 seconds, with the history.
- "Laser com defeito": the job ends in "Falhou — A impressora informou um erro."
- "Cancelar impressão" while waiting: "Cancelado".
- DevTools → device toolbar at 390 px: single column, no horizontal scroll.
