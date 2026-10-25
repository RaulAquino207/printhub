// Package mock is a simulated printing provider for development and tests.
// Nothing it reports comes from real hardware.
package mock

import (
	"context"
	"errors"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// ErrUnknownDestination is returned for a destination the mock does not know.
var ErrUnknownDestination = errors.New("mock: unknown destination")

// Printer is the simulated state behind one destination.
type Printer struct {
	Destination  string
	Status       printer.Status
	Capabilities printer.Capabilities
}

// Provider serves the state of simulated printers.
type Provider struct {
	printers map[string]Printer
}

// New returns a Provider that simulates the given printers.
func New(printers ...Printer) *Provider {
	byDestination := make(map[string]Printer, len(printers))
	for _, p := range printers {
		byDestination[p.Destination] = p
	}
	return &Provider{printers: byDestination}
}

// Status returns the simulated status of the destination.
func (p *Provider) Status(_ context.Context, destination string) (printer.Status, error) {
	sp, ok := p.printers[destination]
	if !ok {
		return "", ErrUnknownDestination
	}
	return sp.Status, nil
}

// Capabilities returns the simulated capabilities of the destination.
func (p *Provider) Capabilities(_ context.Context, destination string) (printer.Capabilities, error) {
	sp, ok := p.printers[destination]
	if !ok {
		return printer.Capabilities{}, ErrUnknownDestination
	}
	return sp.Capabilities, nil
}
