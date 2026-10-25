package printer

import (
	"errors"
	"slices"
	"testing"
)

var monoCaps = Capabilities{
	ColorModes:   []ColorMode{ColorModeMonochrome},
	PaperSizes:   []PaperSize{"A4", "Letter"},
	Orientations: []Orientation{OrientationPortrait, OrientationLandscape},
	PageRanges:   true,
	MaxCopies:    10,
}

func validOptions() Options {
	return Options{
		Copies:      1,
		ColorMode:   ColorModeMonochrome,
		PaperSize:   "A4",
		Orientation: OrientationPortrait,
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Options)
		caps   Capabilities
		want   []FieldError // nil means valid
	}{
		{name: "valid", modify: func(*Options) {}, caps: monoCaps},
		{name: "page ranges", modify: func(o *Options) { o.PageRanges = "1-3, 5" }, caps: monoCaps},
		{name: "max copies", modify: func(o *Options) { o.Copies = 10 }, caps: monoCaps},
		{
			name:   "zero copies",
			modify: func(o *Options) { o.Copies = 0 },
			caps:   monoCaps,
			want:   []FieldError{{"copies", ReasonOutOfRange}},
		},
		{
			name:   "too many copies",
			modify: func(o *Options) { o.Copies = 11 },
			caps:   monoCaps,
			want:   []FieldError{{"copies", ReasonOutOfRange}},
		},
		{
			name:   "color on a monochrome printer",
			modify: func(o *Options) { o.ColorMode = ColorModeColor },
			caps:   monoCaps,
			want:   []FieldError{{"colorMode", ReasonUnsupported}},
		},
		{
			name:   "unsupported paper size",
			modify: func(o *Options) { o.PaperSize = "A3" },
			caps:   monoCaps,
			want:   []FieldError{{"paperSize", ReasonUnsupported}},
		},
		{
			name:   "missing orientation",
			modify: func(o *Options) { o.Orientation = "" },
			caps:   monoCaps,
			want:   []FieldError{{"orientation", ReasonUnsupported}},
		},
		{
			name:   "duplex not supported",
			modify: func(o *Options) { o.Duplex = true },
			caps:   monoCaps,
			want:   []FieldError{{"duplex", ReasonUnsupported}},
		},
		{
			name:   "malformed page ranges",
			modify: func(o *Options) { o.PageRanges = "3-1" },
			caps:   monoCaps,
			want:   []FieldError{{"pageRanges", ReasonInvalidFormat}},
		},
		{
			name:   "page ranges not supported",
			modify: func(o *Options) { o.PageRanges = "1" },
			caps:   Capabilities{ColorModes: monoCaps.ColorModes, PaperSizes: monoCaps.PaperSizes, Orientations: monoCaps.Orientations, MaxCopies: 1},
			want:   []FieldError{{"pageRanges", ReasonUnsupported}},
		},
		{
			name: "several problems at once",
			modify: func(o *Options) {
				o.Copies = 0
				o.ColorMode = ColorModeColor
			},
			caps: monoCaps,
			want: []FieldError{{"copies", ReasonOutOfRange}, {"colorMode", ReasonUnsupported}},
		},
		{
			name:   "unknown capabilities",
			modify: func(*Options) {},
			caps:   Capabilities{},
			want:   []FieldError{{"printer", ReasonUnknown}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			tt.modify(&opts)

			err := opts.Validate(tt.caps)

			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			var optsErr *OptionsError
			if !errors.As(err, &optsErr) {
				t.Fatalf("Validate() = %v, want *OptionsError", err)
			}
			if !slices.Equal(optsErr.Fields, tt.want) {
				t.Errorf("fields = %v, want %v", optsErr.Fields, tt.want)
			}
		})
	}
}

func TestParsePageRanges(t *testing.T) {
	tests := []struct {
		in      string
		want    []PageRange
		wantErr bool
	}{
		{in: "1", want: []PageRange{{1, 1}}},
		{in: "1-3,5", want: []PageRange{{1, 3}, {5, 5}}},
		{in: " 2 - 4 , 7 ", want: []PageRange{{2, 4}, {7, 7}}},
		{in: "", wantErr: true},
		{in: "0", wantErr: true},
		{in: "3-1", wantErr: true},
		{in: "1-", wantErr: true},
		{in: "a-b", wantErr: true},
		{in: "1,,2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParsePageRanges(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePageRanges(%q) = %v, want an error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePageRanges(%q): %v", tt.in, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParsePageRanges(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
