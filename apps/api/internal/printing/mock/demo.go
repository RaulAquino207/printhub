package mock

import (
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// Demo returns a set of simulated printers for local development. The first
// two mirror the real printers (HP LaserJet Pro M1132 MFP, monochrome, and HP
// Color LaserJet CP1215, color) in name and capabilities, so development looks
// like production; they are still flagged as simulated and their model says
// so. An offline printer and a printer whose jobs always fail cover the other
// states. Jobs take a few seconds, so their progress can be followed.
//
// The IDs are stable: stored jobs reference them across restarts.
func Demo() ([]printer.Config, *Provider) {
	bothOrientations := []printer.Orientation{printer.OrientationPortrait, printer.OrientationLandscape}

	configs := []printer.Config{
		{ID: "mono-laser", Name: "HP LaserJet Pro M1132 MFP", Model: "Simulated (mirrors the real M1132)", Destination: "mock-mono", Simulated: true},
		{ID: "color-laser", Name: "HP Color LaserJet CP1215", Model: "Simulated (mirrors the real CP1215)", Destination: "mock-color", Simulated: true},
		{ID: "inkjet", Name: "Jato de tinta", Model: "Simulated inkjet", Destination: "mock-inkjet", Simulated: true},
		{ID: "faulty-laser", Name: "Laser com defeito", Model: "Simulated laser that always fails", Destination: "mock-faulty", Simulated: true},
	}

	provider := New(
		Printer{
			Destination: "mock-mono",
			Status:      printer.StatusOnline,
			Capabilities: printer.Capabilities{
				ColorModes:   []printer.ColorMode{printer.ColorModeMonochrome},
				PaperSizes:   []printer.PaperSize{"A4", "Letter"},
				Orientations: bothOrientations,
				PageRanges:   true,
				MaxCopies:    99,
			},
		},
		Printer{
			Destination: "mock-color",
			Status:      printer.StatusOnline,
			Capabilities: printer.Capabilities{
				ColorModes:   []printer.ColorMode{printer.ColorModeColor, printer.ColorModeMonochrome},
				PaperSizes:   []printer.PaperSize{"A4", "Letter"},
				Orientations: bothOrientations,
				PageRanges:   true,
				MaxCopies:    99,
			},
		},
		Printer{
			Destination: "mock-inkjet",
			Status:      printer.StatusOffline,
			Capabilities: printer.Capabilities{
				ColorModes:   []printer.ColorMode{printer.ColorModeColor, printer.ColorModeMonochrome},
				PaperSizes:   []printer.PaperSize{"A4", "Letter", "Legal"},
				Orientations: bothOrientations,
				Duplex:       true,
				PageRanges:   true,
				MaxCopies:    50,
			},
		},
		Printer{
			Destination: "mock-faulty",
			Status:      printer.StatusOnline,
			FailJobs:    true,
			Capabilities: printer.Capabilities{
				ColorModes:   []printer.ColorMode{printer.ColorModeMonochrome},
				PaperSizes:   []printer.PaperSize{"A4"},
				Orientations: bothOrientations,
				PageRanges:   true,
				MaxCopies:    10,
			},
		},
	)
	provider.SetTiming(Timing{Queue: 2 * time.Second, Print: 5 * time.Second})

	return configs, provider
}
