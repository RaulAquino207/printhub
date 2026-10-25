package mock

import "github.com/RaulAquino207/printhub/apps/api/internal/printer"

// Demo returns a set of simulated printers for local development: a
// monochrome laser, a color laser and an offline printer, so the UI can show
// different capabilities and states. They are generic on purpose and do not
// describe any real device.
func Demo() ([]printer.Config, *Provider) {
	bothOrientations := []printer.Orientation{printer.OrientationPortrait, printer.OrientationLandscape}

	configs := []printer.Config{
		{ID: "mono-laser", Name: "Laser P&B", Model: "Simulated monochrome laser", Destination: "mock-mono", Simulated: true},
		{ID: "color-laser", Name: "Laser colorida", Model: "Simulated color laser", Destination: "mock-color", Simulated: true},
		{ID: "inkjet", Name: "Jato de tinta", Model: "Simulated inkjet", Destination: "mock-inkjet", Simulated: true},
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
	)

	return configs, provider
}
