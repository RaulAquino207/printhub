// Package printer models the printers PrintHub can send jobs to and what each
// of them supports.
package printer

// ID identifies a printer inside PrintHub. It is independent of any provider
// identifier, so a printer keeps its ID if it moves to another queue.
type ID string

// Status is the availability of a printer as reported by its provider.
type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	// StatusUnknown means the provider could not be asked or did not answer.
	StatusUnknown Status = "unknown"
)

// ColorMode is an output color mode a printer can produce.
type ColorMode string

const (
	ColorModeMonochrome ColorMode = "monochrome"
	ColorModeColor      ColorMode = "color"
)

// Orientation is a page orientation.
type Orientation string

const (
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
)

// PaperSize is a media size name, such as "A4" or "Letter".
type PaperSize string

// Capabilities describes what a printer supports. Values come from the
// printing provider; business logic must never infer them from a model name.
type Capabilities struct {
	ColorModes   []ColorMode
	PaperSizes   []PaperSize
	Orientations []Orientation
	Duplex       bool
	PageRanges   bool
	MaxCopies    int
}

// Config is how a printer is registered in PrintHub.
type Config struct {
	ID    ID
	Name  string
	Model string
	// Destination is the provider-specific target, e.g. a CUPS queue name.
	Destination string
	// Simulated marks printers served by the mock provider, so they are never
	// presented as real hardware.
	Simulated bool
}

// Printer is a registered printer together with its current state.
//
// Config is embedded: its fields are promoted, so p.Name works the same as
// p.Config.Name. This is composition, not inheritance.
type Printer struct {
	Config
	Status       Status
	Capabilities Capabilities
}
