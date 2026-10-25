# Structs, interfaces and composition in Go

How PrintHub models printers and plugs in a simulated provider without classes, inheritance or a DI container.

## 1. The problem

The API must list printers whose live state (status, capabilities) comes from a printing provider. Today that provider is simulated; later it will be CUPS. The code that lists printers must not care which one it talks to, and tests must be able to substitute a fake.

## 2. The theory

- **Structs** group fields. There are no classes and no constructors in the language; a `NewX` function returning a struct (or a pointer to one) is a convention.
- **Methods** are functions with a **receiver**: `func (r *Registry) List(...)`.
  - **Pointer receiver** (`*Registry`): the method sees the original value and can modify it; no copy is made. Use it for types that hold state or resources, or are large.
  - **Value receiver** (`fakeProvider`): the method works on a copy. Fine for small, immutable values.
  - Be consistent per type.
- **Named types** (`type ID string`, `type Status string`) give meaning and type safety to primitive values: a `printer.ID` cannot be passed where a `printer.Status` is expected by accident.
- **Interfaces** list method signatures. A type satisfies an interface **implicitly**, just by having the methods — there is no `implements` keyword. This is why idiomatic Go defines interfaces **where they are consumed**, keeps them small, and lets implementations live elsewhere without importing the interface.
- **Embedding** a struct inside another promotes its fields and methods: with `Printer{Config: cfg}`, `p.Name` reads `p.Config.Name`. This is composition, not inheritance: a `Printer` is not a `Config`, and cannot be passed where a `Config` is expected.
- **Sentinel errors** (`var ErrNotFound = errors.New(...)`) are package-level values callers can test with `errors.Is`.

## 3. The chosen solution

```text
internal/printer          domain types + Registry + the Provider interface it consumes
internal/printing/mock    a simulated provider (does not import any interface)
internal/httpapi          HTTP handlers + the PrinterDirectory interface they consume
cmd/printhub-api          wires mock.Demo() → printer.NewRegistry → httpapi.NewRouter
```

- `printer.Provider` has only the two methods the registry needs (`Status`, `Capabilities`). Job submission and cancellation will be defined by the job package when it needs them.
- `httpapi.PrinterDirectory` describes what handlers need (`List`, `Get`); `*printer.Registry` satisfies it, and tests use a `fakeDirectory`.
- Handlers are methods on a `server` struct holding their dependencies, built from an explicit `Dependencies` struct passed by `main`.
- The JSON contract uses dedicated response structs, separate from the domain, so renaming a domain field cannot silently change the API.

## 4. Example from our project

The interface lives with its consumer:

```go
// internal/printer/registry.go
type Provider interface {
	Status(ctx context.Context, destination string) (Status, error)
	Capabilities(ctx context.Context, destination string) (Capabilities, error)
}
```

The mock satisfies it without mentioning it; a test line turns that into a compile-time guarantee:

```go
// internal/printing/mock/mock_test.go
var _ printer.Provider = (*Provider)(nil)
```

Embedding:

```go
type Printer struct {
	Config            // embedded: p.Name, p.ID, p.Simulated are promoted
	Status       Status
	Capabilities Capabilities
}
```

Graceful degradation instead of failing everything:

```go
status, err := r.provider.Status(ctx, cfg.Destination)
if err != nil {
	return p // listed with StatusUnknown and no capabilities
}
```

`main` wires everything explicitly — this is the whole "DI container":

```go
printerConfigs, provider := mock.Demo()
printers := printer.NewRegistry(printerConfigs, provider)
handler := httpapi.NewRouter(httpapi.Dependencies{Logger: logger, CORSOrigins: cfg.corsOrigins, Printers: printers})
```

## 5. Alternatives and trade-offs

| NestJS habit | Go approach here | Why |
|---|---|---|
| `@Injectable()` services resolved by a container | Constructors + explicit wiring in `main` | Dependencies are visible in one place; no reflection or decorators |
| Interface declared next to the implementation (`implements`) | Interface declared by the consumer, satisfied implicitly | Consumers ask only for what they use; implementations stay decoupled |
| Class inheritance (`extends`) | Struct embedding | Reuse of fields without an "is-a" relationship |
| DTO classes with validation decorators | Plain response structs with `json` tags | Explicit mapping; validation comes as code in later milestones |
| Mocking libraries (`jest.fn()`) | Hand-written fakes | Any type with the right methods is a valid test double |

Trade-off: the response structs duplicate some domain fields. That small duplication buys a stable API contract.

## 6. How to validate

```bash
moon run api:test                            # registry, mock and handler tests
moon run api:dev
curl -s localhost:8080/api/v1/printers       # three simulated printers, "simulated": true
curl -si localhost:8080/api/v1/printers/nope # 404 {"error":{"code":"printer_not_found",...}}
```
