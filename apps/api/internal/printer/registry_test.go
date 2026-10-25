package printer

import (
	"context"
	"errors"
	"testing"
)

// fakeProvider is a hand-written test double: any type with the right methods
// satisfies Provider, so no mocking library is needed.
type fakeProvider struct {
	status  map[string]Status
	failFor string
}

func (f fakeProvider) Status(_ context.Context, destination string) (Status, error) {
	if destination == f.failFor {
		return "", errors.New("provider unreachable")
	}
	return f.status[destination], nil
}

func (f fakeProvider) Capabilities(_ context.Context, destination string) (Capabilities, error) {
	if destination == f.failFor {
		return Capabilities{}, errors.New("provider unreachable")
	}
	return Capabilities{ColorModes: []ColorMode{ColorModeMonochrome}, MaxCopies: 1}, nil
}

func newTestRegistry() *Registry {
	configs := []Config{
		{ID: "a", Name: "Printer A", Destination: "queue-a"},
		{ID: "b", Name: "Printer B", Destination: "queue-b"},
	}
	provider := fakeProvider{
		status:  map[string]Status{"queue-a": StatusOnline, "queue-b": StatusOffline},
		failFor: "queue-b",
	}
	return NewRegistry(configs, provider)
}

func TestRegistryList(t *testing.T) {
	printers, err := newTestRegistry().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(printers) != 2 {
		t.Fatalf("got %d printers, want 2", len(printers))
	}

	a := printers[0]
	if a.ID != "a" || a.Status != StatusOnline || a.Capabilities.MaxCopies != 1 {
		t.Errorf("printer a = %+v, want online with capabilities", a)
	}

	// The provider fails for b: it must still be listed, as unknown.
	b := printers[1]
	if b.ID != "b" || b.Status != StatusUnknown || b.Capabilities.MaxCopies != 0 {
		t.Errorf("printer b = %+v, want unknown without capabilities", b)
	}
}

func TestRegistryListCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := newTestRegistry().List(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRegistryGet(t *testing.T) {
	reg := newTestRegistry()

	p, err := reg.Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get(a): %v", err)
	}
	if p.Name != "Printer A" {
		t.Errorf("Name = %q, want %q", p.Name, "Printer A")
	}

	if _, err := reg.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrNotFound", err)
	}
}
