package printer

import (
	"context"
	"errors"
)

// ErrNotFound is returned when no printer is registered with the given ID.
var ErrNotFound = errors.New("printer not found")

// Provider reports the live state of printers. It is defined here, where it is
// consumed, and kept to the two methods the registry needs; implementations
// (mock, CUPS) live in their own packages and satisfy it implicitly.
type Provider interface {
	Status(ctx context.Context, destination string) (Status, error)
	Capabilities(ctx context.Context, destination string) (Capabilities, error)
}

// Registry combines the registered printers with the state reported by a
// Provider.
type Registry struct {
	configs  []Config
	provider Provider
}

// NewRegistry returns a Registry for the given printers and provider.
func NewRegistry(configs []Config, provider Provider) *Registry {
	return &Registry{configs: configs, provider: provider}
}

// List returns every registered printer with its current state.
func (r *Registry) List(ctx context.Context) ([]Printer, error) {
	printers := make([]Printer, 0, len(r.configs))
	for _, cfg := range r.configs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		printers = append(printers, r.describe(ctx, cfg))
	}
	return printers, nil
}

// Get returns the printer with the given ID, or ErrNotFound.
func (r *Registry) Get(ctx context.Context, id ID) (Printer, error) {
	for _, cfg := range r.configs {
		if cfg.ID == id {
			return r.describe(ctx, cfg), nil
		}
	}
	return Printer{}, ErrNotFound
}

// describe asks the provider for the printer's state. A provider failure does
// not hide the printer: it is reported with StatusUnknown and no capabilities,
// so one unreachable printer cannot break the whole list.
func (r *Registry) describe(ctx context.Context, cfg Config) Printer {
	p := Printer{Config: cfg, Status: StatusUnknown}

	status, err := r.provider.Status(ctx, cfg.Destination)
	if err != nil {
		return p
	}
	caps, err := r.provider.Capabilities(ctx, cfg.Destination)
	if err != nil {
		return p
	}

	p.Status = status
	p.Capabilities = caps
	return p
}
