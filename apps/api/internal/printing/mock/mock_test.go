package mock

import (
	"context"
	"errors"
	"slices"
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

// Stored jobs reference printers by ID, so these IDs must survive restarts and
// refactors. Changing one orphans the jobs that point to it: treat any change
// here as a data migration, not a rename.
func TestDemoIDsAreStable(t *testing.T) {
	configs, _ := Demo()

	var ids []printer.ID
	for _, cfg := range configs {
		ids = append(ids, cfg.ID)
	}

	want := []printer.ID{"mono-laser", "color-laser", "inkjet"}
	if !slices.Equal(ids, want) {
		t.Errorf("demo printer IDs = %v, want %v", ids, want)
	}
}
