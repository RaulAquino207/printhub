package printer

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Options are the print settings chosen for a job.
type Options struct {
	Copies      int
	ColorMode   ColorMode
	PaperSize   PaperSize
	Orientation Orientation
	Duplex      bool
	// PageRanges selects pages, e.g. "1-3,5". Empty means every page.
	PageRanges string
}

// Reasons reported in a FieldError. They are stable codes, not messages.
const (
	ReasonUnsupported   = "unsupported"
	ReasonOutOfRange    = "out_of_range"
	ReasonInvalidFormat = "invalid_format"
	ReasonUnknown       = "capabilities_unknown"
)

// FieldError reports one invalid option.
type FieldError struct {
	Field  string // JSON-style field name, e.g. "colorMode"
	Reason string // one of the Reason* constants
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// OptionsError lists every invalid option found by Validate. Callers retrieve
// it with errors.As to report all problems at once.
type OptionsError struct {
	Fields []FieldError
}

func (e *OptionsError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Error()
	}
	return "invalid print options: " + strings.Join(parts, ", ")
}

// Validate checks the options against what a printer supports. It returns nil
// or an *OptionsError.
func (o Options) Validate(c Capabilities) error {
	// Without reported capabilities nothing can be checked, so nothing is accepted.
	if len(c.ColorModes) == 0 {
		return &OptionsError{Fields: []FieldError{{Field: "printer", Reason: ReasonUnknown}}}
	}

	var fields []FieldError
	add := func(field, reason string) {
		fields = append(fields, FieldError{Field: field, Reason: reason})
	}

	if o.Copies < 1 || o.Copies > c.MaxCopies {
		add("copies", ReasonOutOfRange)
	}
	if !slices.Contains(c.ColorModes, o.ColorMode) {
		add("colorMode", ReasonUnsupported)
	}
	if !slices.Contains(c.PaperSizes, o.PaperSize) {
		add("paperSize", ReasonUnsupported)
	}
	if !slices.Contains(c.Orientations, o.Orientation) {
		add("orientation", ReasonUnsupported)
	}
	if o.Duplex && !c.Duplex {
		add("duplex", ReasonUnsupported)
	}
	if o.PageRanges != "" {
		if !c.PageRanges {
			add("pageRanges", ReasonUnsupported)
		} else if _, err := ParsePageRanges(o.PageRanges); err != nil {
			add("pageRanges", ReasonInvalidFormat)
		}
	}

	// Return a literal nil: returning a nil *OptionsError would produce a
	// non-nil error interface.
	if len(fields) == 0 {
		return nil
	}
	return &OptionsError{Fields: fields}
}

// PageRange is an inclusive range of 1-based page numbers.
type PageRange struct {
	First, Last int
}

// ParsePageRanges parses a list such as "1-3, 5". Pages start at 1 and each
// range must be ascending. Whether the pages exist depends on the document and
// is checked later.
func ParsePageRanges(s string) ([]PageRange, error) {
	var ranges []PageRange
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		first, last, isRange := strings.Cut(part, "-")

		start, err := parsePage(first)
		if err != nil {
			return nil, fmt.Errorf("page range %q: %w", part, err)
		}
		end := start
		if isRange {
			if end, err = parsePage(last); err != nil {
				return nil, fmt.Errorf("page range %q: %w", part, err)
			}
		}
		if end < start {
			return nil, fmt.Errorf("page range %q: end before start", part)
		}
		ranges = append(ranges, PageRange{First: start, Last: end})
	}
	return ranges, nil
}

func parsePage(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("not a page number: %q", s)
	}
	if n < 1 {
		return 0, fmt.Errorf("pages start at 1, got %d", n)
	}
	return n, nil
}
