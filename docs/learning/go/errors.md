# Errors in Go

How PrintHub reports invalid print options and invalid job transitions without exceptions.

## 1. The problem

Two pieces of pure domain logic must reject bad input with enough detail for callers to react:

- Print options checked against a printer's capabilities: the API must tell the web **every** invalid field (copies out of range, color on a monochrome printer, …).
- Job status changes: a cancelled job must never start processing, and the caller must know which transition was refused.

## 2. The theory

- In Go, **errors are values**. A function returns `error` as its last result; the caller checks it with `if err != nil`. There is no `throw`/`catch`: control flow stays visible.
- `error` is an interface with a single method: `Error() string`. Any type with that method is an error, so errors can carry structured data.
- **Sentinel errors** are package-level values compared by identity: `var ErrNotFound = errors.New("printer not found")`.
- **Wrapping**: `fmt.Errorf("listen: %w", err)` adds context while keeping the original error reachable. A custom type can also expose an inner error through an `Unwrap() error` method.
- **`errors.Is(err, target)`** walks the wrap chain looking for a value equal to `target` — use it for sentinels.
- **`errors.As(err, &target)`** walks the chain looking for a value of `target`'s type and assigns it — use it to read the fields of a custom error type.
- **The typed-nil pitfall**: an interface value is nil only if both its type and value are nil. Returning a nil `*OptionsError` as `error` produces a **non-nil** error. Always `return nil` explicitly.

## 3. The chosen solution

| Case | Error design | How callers use it |
|---|---|---|
| Printer not found | Sentinel `printer.ErrNotFound` | `errors.Is(err, printer.ErrNotFound)` → HTTP 404 |
| Invalid options | `*printer.OptionsError{Fields []FieldError}` with stable reason codes | `errors.As(err, &optsErr)` → list every field in the response |
| Invalid transition | `*job.TransitionError{From, To}` whose `Unwrap` returns `job.ErrInvalidTransition` | `errors.Is` to detect it, `errors.As` to read `From`/`To` |

Reason codes (`unsupported`, `out_of_range`, `invalid_format`, `capabilities_unknown`) are stable strings for programs; the web maps them to pt-BR messages.

## 4. Example from our project

Collecting every problem, then returning a literal `nil` when there are none:

```go
// internal/printer/options.go
var fields []FieldError
if o.Copies < 1 || o.Copies > c.MaxCopies {
	fields = append(fields, FieldError{Field: "copies", Reason: ReasonOutOfRange})
}
// ... other checks ...
if len(fields) == 0 {
	return nil // not a nil *OptionsError
}
return &OptionsError{Fields: fields}
```

A custom error that also matches a sentinel:

```go
// internal/job/job.go
var ErrInvalidTransition = errors.New("invalid job status transition")

type TransitionError struct{ From, To Status }

func (e *TransitionError) Error() string { return fmt.Sprintf("job: cannot move from %s to %s", e.From, e.To) }
func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }
```

Using both checks:

```go
err := j.StartProcessing(now)
if errors.Is(err, job.ErrInvalidTransition) { /* e.g. HTTP 409 Conflict */ }

var te *job.TransitionError
if errors.As(err, &te) {
	log.Printf("refused %s → %s", te.From, te.To)
}
```

The job's state changes in exactly one method (`moveTo`), which validates first and mutates second, so a rejected transition leaves the job untouched.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| NestJS-style exceptions + exception filters | Not available in Go; `panic` exists but is reserved for programmer bugs, not expected failures |
| Return only the first invalid field | Simpler, but the user would fix fields one round-trip at a time |
| `errors.Join` of field errors | Standard library, but callers would need to unwrap a list to rebuild field/reason pairs; a dedicated type is clearer |
| String matching on `err.Error()` | Fragile: messages are for humans and may change |

## 6. How to validate

```bash
moon run api:test
go -C apps/api test -cover ./internal/job ./internal/printer
# job: every (from, to) status pair is checked against the expected lifecycle
# printer: table-driven tests for each invalid option and for page range parsing
```
