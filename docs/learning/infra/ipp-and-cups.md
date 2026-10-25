# IPP and CUPS

How the PrintHub API talks to a real print server, and how it is tested without a printer.

## 1. The problem

Simulated printers proved the application flow, but real printing goes through CUPS — on the Raspberry Pi next to the HP printers. The API must discover queues, read what each one supports, submit PDFs with the chosen options, follow jobs and cancel them, all over the network. And this has to be developed and tested before any hardware is available.

## 2. The theory

- **CUPS** (Common UNIX Printing System) is the print server on Linux and macOS. It keeps **queues** (one per printer), converts documents with **filters**, and sends the result to the device through a **backend** (`usb://…`, `ipp://…`, `file:///…`).
- **IPP** (Internet Printing Protocol, RFC 8010/8011) is how clients talk to CUPS. Messages are binary — a version, an operation or status code, a request ID, then **attribute groups** (operation, job, printer) — sent as HTTP `POST` with `Content-Type: application/ipp`. For `Print-Job`, the document bytes simply follow the IPP message in the same body.
- **Attributes** are typed and multi-valued: `copies` (integer), `print-color-mode` (keyword), `orientation-requested` (enum: 3 portrait, 4 landscape), `page-ranges` (1setOf rangeOfInteger), `media` (keyword such as `iso_a4_210x297mm`).
- **`*-supported` attributes** describe a printer's capabilities: `print-color-mode-supported`, `media-supported`, `sides-supported`, `copies-supported` (a range)… This is how PrintHub learns capabilities instead of hardcoding printer models.
- **States**: `printer-state` 3 idle, 4 processing, 5 stopped; `job-state` 3 pending, 4 held, 5 processing, 6 processing-stopped, 7 canceled, 8 aborted, 9 completed.
- **PPD files** describe a printer to CUPS (sizes, color, duplex, filters). CUPS derives the IPP `*-supported` attributes from them.
- **"Completed" is not "printed"**: CUPS marks a job completed when the backend accepted all the data. In the development container `job-media-sheets-completed` stays `0` for completed jobs.

## 3. The chosen solution

**Library**: [OpenPrinting/goipp](https://github.com/OpenPrinting/goipp) — the OpenPrinting project's protocol layer (encoding and decoding only), on top of Go's `net/http`. A higher-level client (`phin1x/go-ipp`) existed but had no release since 2020; owning the five operations we need keeps the code small and understandable.

**Provider** (`apps/api/internal/printing/cups`):

| PrintHub need | IPP operation | Mapping |
|---|---|---|
| Printers | `CUPS-Get-Printers` | queue name → printer ID (stable), `printer-info` → name, `printer-make-and-model` → model |
| Status | `Get-Printer-Attributes` | online if `printer-is-accepting-jobs` and `printer-state` ≠ stopped |
| Capabilities | `Get-Printer-Attributes` | `*-supported` attributes; unknown media ignored; monochrome always available |
| Submit | `Print-Job` | document streamed after the IPP message; options → `copies`, `print-color-mode`, `media`, `orientation-requested`, `sides`, `page-ranges` |
| Follow | `Get-Job-Attributes` | `job-state` → pending / processing / completed / failed / cancelled |
| Cancel | `Cancel-Job` | `not-possible` → already finished; `not-found` → unknown job |

Each call has a 10-second deadline, except the upload, which only follows the request context.

**Selection** (`cmd/printhub-api`): `PRINTHUB_PRINTING_PROVIDER=mock|cups`. With `cups`, `PRINTHUB_CUPS_URL` points at the server and the queues are discovered at startup. `PRINTHUB_CUPS_SIMULATED=true` marks every printer as simulated — mandatory for the development container, so the UI never presents its queues as real hardware.

**Development CUPS** (`deploy/dev`): a Debian container with CUPS and three virtual queues writing to `/dev/null`, defined by minimal PPDs that accept PDF without any filter:

| Queue | Purpose |
|---|---|
| `virtual-mono` | monochrome, A4/Letter |
| `virtual-color` | color + duplex |
| `virtual-paused` | disabled: jobs stay pending (cancellation tests); shows as offline |

It is published on `127.0.0.1:6631` only, because its configuration accepts any request without authentication.

## 4. Example from our project

Streaming the PDF after the IPP message, without buffering it (`cups.go`):

```go
payload, _ := req.EncodeBytes()                          // the IPP message
body := io.MultiReader(bytes.NewReader(payload), doc)    // + the document
httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
httpReq.Header.Set("Content-Type", goipp.ContentType)
```

Page ranges as IPP ranges:

```go
values[i] = goipp.Range{Lower: r.First, Upper: r.Last}
attrs = append(attrs, goipp.MakeAttr("page-ranges", goipp.TagRange, values[0], values[1:]...))
```

Composing the two consumer interfaces in `main`, so one value must satisfy both:

```go
type printingProvider interface {
	printer.Provider // Status, Capabilities
	worker.Provider  // Submit, JobStatus, Cancel
}
```

What CUPS recorded for a PrintHub job (`ipptool … get-job-attributes.test` inside the container): `copies = 2`, `print-color-mode = monochrome`, `media = iso_a4_210x297mm`, `orientation-requested = portrait`, `sides = one-sided`, `page-ranges = 1-1`, `job-state = completed`, `job-media-sheets-completed = 0`.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| Shelling out to `lp` / `lpstat` | Simple, but needs CUPS client tools in the API image and parsing human-oriented output |
| `phin1x/go-ipp` | Higher-level API, but unmaintained since 2020 |
| libcups via cgo | The reference implementation, but cgo would undo the pure-Go build |
| IPP Everywhere directly to printers | Modern printers speak IPP natively; the HP M1132 and CP1215 do not, so a CUPS host with drivers stays necessary |
| Static printer list in configuration | Explicit, but duplicates what CUPS already knows; discovery uses queue names, which are stable |

Limitation: printers are discovered once at startup; adding a CUPS queue requires restarting the API.

## 6. How to validate

```bash
moon run api:cups-up                 # dev CUPS on http://localhost:6631 (web UI there too)
moon run api:test-integration        # discovery, capabilities, print, cancel, errors against real CUPS
moon run api:test                    # unit tests with a fake IPP server, no Docker needed

PRINTHUB_PRINTING_PROVIDER=cups PRINTHUB_CUPS_URL=http://localhost:6631 PRINTHUB_CUPS_SIMULATED=true \
  moon run api:dev web:dev           # the UI lists the virtual queues, marked "Simulada"

docker exec printhub-dev-cups lpstat -W all -o   # jobs as CUPS sees them
moon run api:cups-down
```
