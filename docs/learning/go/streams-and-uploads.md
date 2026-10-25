# Streams and safe uploads in Go

How `POST /api/v1/jobs` receives a PDF of up to 50 MB without loading it into memory, and why every upload is treated as hostile.

## 1. The problem

Creating a job means receiving a file plus some fields from a browser. The file can be large, and nothing about it can be trusted: not its size, not its declared type, not its name. A naive handler that reads the whole body into memory, trusts the `.pdf` extension and saves the file under the client's filename is vulnerable to memory exhaustion, disk filling, path traversal (`../../etc/passwd`) and arbitrary content reaching the print pipeline.

## 2. The theory

- **`io.Reader` and `io.Writer`** are Go's two central interfaces: anything that produces bytes (`Read(p []byte)`) or consumes them (`Write(p []byte)`). Files, network bodies, buffers and compressors all implement them, so they compose freely.
- **`io.Copy(dst, src)`** moves bytes from a reader to a writer through a small buffer: memory use stays constant whatever the size.
- **Wrapping readers** adds behavior without loading data:
  - `io.LimitReader(r, n)` stops after `n` bytes;
  - `http.MaxBytesReader(w, body, n)` fails once a request body exceeds `n` bytes and tells the server to close the connection;
  - `bufio.Reader.Peek(n)` looks at the next bytes without consuming them.
- **Multipart streaming**: `r.MultipartReader()` returns the parts of a `multipart/form-data` body one at a time, as readers. `r.ParseMultipartForm`, by contrast, reads everything first (spilling to temporary files).
- **Magic bytes**: file formats start with a signature. Every PDF begins with `%PDF-`. Extensions and `Content-Type` headers are set by the client and prove nothing.
- **Atomic writes**: write to a temporary file in the same directory, `Sync` it to disk, then `Rename` it into place. A rename within one filesystem is atomic, so readers never see half a file.
- **`defer` with named results**: a deferred function runs when the surrounding function returns and can read the named result `err` — the idiomatic way to clean up only on failure.

## 3. The chosen solution

Two layers with separate responsibilities:

**`internal/document.Storage`** (knows nothing about HTTP):

- Refuses IDs outside `[A-Za-z0-9_-]{1,64}`, so a document path can never leave its directory.
- Peeks the first bytes: anything not starting with `%PDF-` is `ErrNotPDF`.
- Copies through `io.LimitReader(max+1)` into a `0600` temporary file; reading one byte past the limit proves the file is too large (`ErrTooLarge`) without trusting any declared size.
- `Sync` + `Close` + `Rename` to `<id>.pdf`; on any failure the deferred cleanup removes the temporary file.

**`POST /api/v1/jobs` handler** (`internal/httpapi/jobs.go`):

- Caps the whole body with `http.MaxBytesReader` (file limit + 1 MiB for the other parts).
- Streams parts in any order. Small fields (`printerId`, `options`) are read with a 64 KiB cap; the `file` part goes straight to `Storage.Save`.
- Generates the job ID first with `crypto/rand.Text()` (26 random base32 characters from the standard library), and stores the file under that ID. The client's filename is cleaned (no path, no control characters, at most 200 characters) and kept **only as metadata**.
- Collects every problem and answers `422` with all invalid fields at once; a too-large body answers `413`.
- A deferred cleanup removes the stored file unless the job was saved, so validation errors and database failures leave nothing behind.

| Situation | Status | Body |
|---|---|---|
| Created | `201` + `Location: /api/v1/jobs/{id}` | The job |
| Not multipart | `400` | `invalid_request` |
| File over the limit (default 50 MiB, `PRINTHUB_MAX_UPLOAD_MB`) | `413` | `file_too_large` |
| Not a PDF, missing fields, unknown or offline printer, unsupported options | `422` | `validation_failed` + `fields` |

## 4. Example from our project

Cleaning up only on failure, using a named result:

```go
func (s *Storage) Save(id string, r io.Reader) (size int64, err error) {
	tmp, err := os.CreateTemp(s.dir, "upload-*.tmp")
	// ...
	defer func() {
		if err != nil { // sees the error being returned
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	size, err = io.Copy(tmp, io.LimitReader(br, s.maxBytes+1))
	if size > s.maxBytes {
		return 0, ErrTooLarge // triggers the cleanup
	}
	// ...
}
```

The handler's cleanup flag:

```go
defer func() {
	if fileStored && !jobSaved {
		s.documents.Remove(string(id))
	}
}()
```

A Go detail worth noticing: `optionsRequest` (the JSON contract, with tags) and `printer.Options` (the domain type) have the same fields in the same order, so one converts to the other directly — `printer.Options(req)`. Struct tags are ignored by conversions.

Validation errors, all at once:

```json
{"error":{"code":"validation_failed","message":"the request has invalid fields",
  "fields":[{"field":"copies","reason":"out_of_range"},{"field":"colorMode","reason":"unsupported"}]}}
```

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| `r.ParseMultipartForm` + `r.FormFile` | Simpler code, but the whole body is read before any check, and Go spills it to its own temporary files |
| Trusting `Content-Type` or the extension | Trivially spoofed (the HTML file named `fake.pdf` in the milestone 7 test) |
| Full PDF parsing (page count, structure) | Catches malformed PDFs earlier and enables page-range checks, but parsers are a large attack surface; deferred until needed |
| Storing files under the original name | Readable on disk, but collisions and path traversal; the ID-based name avoids both |
| Storing files in SQLite (BLOB) | One place to back up, but bloats the database and complicates streaming |
| `google/uuid` for IDs | Common, but `crypto/rand.Text()` gives the same unguessability from the standard library |

Known limitation: if the process dies between storing the file and saving the job, an orphan file remains. The worker milestone adds cleanup of documents whose job is missing or finished.

## 6. How to validate

```bash
moon run api:test   # document storage + handler tests (order, validation, 413, cleanup, filenames)

OPTS='{"copies":1,"colorMode":"monochrome","paperSize":"A4","orientation":"portrait","duplex":false,"pageRanges":""}'
curl -s -X POST localhost:8080/api/v1/jobs -F printerId=mono-laser -F "options=$OPTS" -F "file=@teste.pdf"
# 201 with the queued job; the file appears as apps/api/data/documents/<id>.pdf (mode 600)

echo '<html></html>' > fake.pdf
curl -s -X POST localhost:8080/api/v1/jobs -F printerId=mono-laser -F "options=$OPTS" -F "file=@fake.pdf"
# 422 {"field":"file","reason":"not_pdf"}, nothing stored
```
