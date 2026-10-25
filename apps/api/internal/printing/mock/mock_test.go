package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// Compile-time check: *Provider must satisfy printer.Provider. If a method is
// missing or has the wrong signature, this line stops the build.
var _ printer.Provider = (*Provider)(nil)

func TestProvider(t *testing.T) {
	ctx := context.Background()
	p := New(Printer{
		Destination:  "q",
		Status:       printer.StatusOffline,
		Capabilities: printer.Capabilities{MaxCopies: 5},
	})

	status, err := p.Status(ctx, "q")
	if err != nil || status != printer.StatusOffline {
		t.Errorf("Status(q) = %q, %v; want offline, nil", status, err)
	}

	caps, err := p.Capabilities(ctx, "q")
	if err != nil || caps.MaxCopies != 5 {
		t.Errorf("Capabilities(q) = %+v, %v; want MaxCopies 5, nil", caps, err)
	}

	if _, err := p.Status(ctx, "missing"); !errors.Is(err, ErrUnknownDestination) {
		t.Errorf("Status(missing) err = %v, want ErrUnknownDestination", err)
	}
	if _, err := p.Capabilities(ctx, "missing"); !errors.Is(err, ErrUnknownDestination) {
		t.Errorf("Capabilities(missing) err = %v, want ErrUnknownDestination", err)
	}
}

func TestDemoConfigsMatchProvider(t *testing.T) {
	configs, provider := Demo()

	for _, cfg := range configs {
		if !cfg.Simulated {
			t.Errorf("%s: demo printer must be marked as simulated", cfg.ID)
		}
		if _, err := provider.Status(context.Background(), cfg.Destination); err != nil {
			t.Errorf("%s: destination %q not simulated: %v", cfg.ID, cfg.Destination, err)
		}
	}
}
