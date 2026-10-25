// Package cups is a printing provider that talks IPP to a CUPS server.
//
// IPP is a binary protocol carried over HTTP POST. goipp encodes and decodes
// the messages; this package builds the few requests PrintHub needs and maps
// IPP attributes to PrintHub's types.
package cups

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/OpenPrinting/goipp"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
)

// requestTimeout bounds every IPP call except document upload.
const requestTimeout = 10 * time.Second

// Provider talks to one CUPS server, e.g. http://localhost:631.
type Provider struct {
	base      *url.URL
	http      *http.Client
	user      string
	simulated bool
	requestID atomic.Uint32
}

// Options configure a Provider.
type Options struct {
	// URL of the CUPS server, e.g. "http://print-bridge:631".
	URL string
	// Simulated marks every printer of this server as not real hardware
	// (e.g. the development CUPS container with virtual queues).
	Simulated bool
}

// New returns a Provider for the CUPS server at opts.URL.
func New(opts Options) (*Provider, error) {
	u, err := url.Parse(opts.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("cups: invalid URL %q (want http://host:port)", opts.URL)
	}
	return &Provider{
		base: u,
		// No overall client timeout: uploads may take long. Each call sets a
		// context deadline instead.
		http:      &http.Client{},
		user:      "printhub",
		simulated: opts.Simulated,
	}, nil
}

// ipp IPP URIs use the ipp:// scheme with the same host and port.
func (p *Provider) ippURI(path string) string {
	scheme := "ipp"
	if p.base.Scheme == "https" {
		scheme = "ipps"
	}
	return scheme + "://" + p.base.Host + path
}

func (p *Provider) newRequest(op goipp.Op) *goipp.Message {
	m := goipp.NewRequest(goipp.DefaultVersion, op, p.requestID.Add(1))
	m.Operation.Add(goipp.MakeAttr("attributes-charset", goipp.TagCharset, goipp.String("utf-8")))
	m.Operation.Add(goipp.MakeAttr("attributes-natural-language", goipp.TagLanguage, goipp.String("en-us")))
	return m
}

// statusError is a non-successful IPP status.
type statusError struct{ status goipp.Status }

func (e statusError) Error() string { return "cups: " + e.status.String() }

// do sends an IPP request to path and returns the decoded response. doc, if
// not nil, is streamed after the IPP message (Print-Job).
func (p *Provider) do(ctx context.Context, path string, req *goipp.Message, doc io.Reader) (*goipp.Message, error) {
	payload, err := req.EncodeBytes()
	if err != nil {
		return nil, fmt.Errorf("cups: encode request: %w", err)
	}
	var body io.Reader = bytes.NewReader(payload)
	if doc != nil {
		body = io.MultiReader(body, doc)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base.JoinPath(path).String(), body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", goipp.ContentType)
	httpReq.Header.Set("Accept", goipp.ContentType)

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cups: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cups: HTTP %s", resp.Status)
	}

	var msg goipp.Message
	if err := msg.Decode(resp.Body); err != nil {
		return nil, fmt.Errorf("cups: decode response: %w", err)
	}
	// Success statuses are 0x0000-0x00ff (e.g. "ok, ignored attributes").
	if status := goipp.Status(msg.Code); status > 0x00ff {
		return nil, statusError{status}
	}
	return &msg, nil
}

func hasStatus(err error, status goipp.Status) bool {
	var se statusError
	return errors.As(err, &se) && se.status == status
}

// PrinterInfo describes a queue found on the server.
type PrinterInfo struct {
	Queue string
	Info  string // printer-info: human description
	Model string // printer-make-and-model
}

// Printers lists the server's queues (CUPS-Get-Printers).
func (p *Provider) Printers(ctx context.Context) ([]PrinterInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req := p.newRequest(goipp.OpCupsGetPrinters)
	req.Operation.Add(goipp.MakeAttr("requested-attributes", goipp.TagKeyword,
		goipp.String("printer-name"), goipp.String("printer-info"), goipp.String("printer-make-and-model")))

	resp, err := p.do(ctx, "/", req, nil)
	if err != nil {
		return nil, err
	}

	var printers []PrinterInfo
	for _, g := range resp.Groups {
		if g.Tag != goipp.TagPrinterGroup {
			continue
		}
		a := attrs(g.Attrs)
		if name := a.string("printer-name"); name != "" {
			printers = append(printers, PrinterInfo{
				Queue: name,
				Info:  a.string("printer-info"),
				Model: a.string("printer-make-and-model"),
			})
		}
	}
	return printers, nil
}

// Configs turns the server's queues into printer configurations. The queue
// name is the printer ID, so it must stay stable once jobs reference it.
func (p *Provider) Configs(ctx context.Context) ([]printer.Config, error) {
	infos, err := p.Printers(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]printer.Config, 0, len(infos))
	for _, info := range infos {
		name := info.Info
		if name == "" {
			name = info.Queue
		}
		configs = append(configs, printer.Config{
			ID:          printer.ID(info.Queue),
			Name:        name,
			Model:       info.Model,
			Destination: info.Queue,
			Simulated:   p.simulated,
		})
	}
	return configs, nil
}

func (p *Provider) printerAttributes(ctx context.Context, queue string, names ...string) (attrs, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req := p.newRequest(goipp.OpGetPrinterAttributes)
	req.Operation.Add(goipp.MakeAttr("printer-uri", goipp.TagURI, goipp.String(p.ippURI("/printers/"+queue))))
	values := make([]goipp.Value, len(names))
	for i, n := range names {
		values[i] = goipp.String(n)
	}
	req.Operation.Add(goipp.MakeAttr("requested-attributes", goipp.TagKeyword, values[0], values[1:]...))

	resp, err := p.do(ctx, "/printers/"+queue, req, nil)
	if err != nil {
		return nil, err
	}
	return attrs(resp.Printer), nil
}

// printerStopped is the IPP printer-state of a paused queue (RFC 8011; idle
// is 3, processing is 4).
const printerStopped = 5

// Status reports online when the queue accepts jobs and is not stopped.
func (p *Provider) Status(ctx context.Context, queue string) (printer.Status, error) {
	a, err := p.printerAttributes(ctx, queue, "printer-state", "printer-is-accepting-jobs")
	if err != nil {
		return "", err
	}
	state, _ := a.int("printer-state")
	if a.bool("printer-is-accepting-jobs") && state != printerStopped {
		return printer.StatusOnline, nil
	}
	return printer.StatusOffline, nil
}

// Capabilities maps the queue's "-supported" attributes.
func (p *Provider) Capabilities(ctx context.Context, queue string) (printer.Capabilities, error) {
	a, err := p.printerAttributes(ctx, queue,
		"print-color-mode-supported", "color-supported", "media-supported",
		"orientation-requested-supported", "sides-supported",
		"copies-supported", "page-ranges-supported")
	if err != nil {
		return printer.Capabilities{}, err
	}
	return capabilitiesFrom(a), nil
}

func capabilitiesFrom(a attrs) printer.Capabilities {
	var c printer.Capabilities

	modes := a.strings("print-color-mode-supported")
	if len(modes) == 0 && a.bool("color-supported") {
		modes = []string{"color", "monochrome"}
	}
	hasColor := false
	for _, m := range modes {
		if m == "color" {
			hasColor = true
		}
	}
	if hasColor {
		c.ColorModes = append(c.ColorModes, printer.ColorModeColor)
	}
	// Every printer can print in black and white.
	c.ColorModes = append(c.ColorModes, printer.ColorModeMonochrome)

	for _, media := range a.strings("media-supported") {
		if size, ok := paperSizeFromMedia[media]; ok {
			c.PaperSizes = append(c.PaperSizes, size)
		}
	}

	orientations := a.ints("orientation-requested-supported")
	if len(orientations) == 0 {
		orientations = []int{orientationPortrait, orientationLandscape}
	}
	for _, o := range orientations {
		switch o {
		case orientationPortrait:
			c.Orientations = append(c.Orientations, printer.OrientationPortrait)
		case orientationLandscape:
			c.Orientations = append(c.Orientations, printer.OrientationLandscape)
		}
	}

	for _, s := range a.strings("sides-supported") {
		if s == sidesTwoSidedLong {
			c.Duplex = true
		}
	}

	c.MaxCopies = 1
	if r, ok := a.rangeUpper("copies-supported"); ok {
		c.MaxCopies = r
	}
	c.PageRanges = a.bool("page-ranges-supported")
	return c
}

// IPP keyword and enum values used in job attributes.
const (
	orientationPortrait  = 3
	orientationLandscape = 4
	sidesOneSided        = "one-sided"
	sidesTwoSidedLong    = "two-sided-long-edge"
)

// mediaFromPaperSize maps PrintHub paper sizes to IPP "media" keywords (PWG 5101.1).
var mediaFromPaperSize = map[printer.PaperSize]string{
	"A4":     "iso_a4_210x297mm",
	"Letter": "na_letter_8.5x11in",
	"Legal":  "na_legal_8.5x14in",
}

var paperSizeFromMedia = func() map[string]printer.PaperSize {
	m := make(map[string]printer.PaperSize, len(mediaFromPaperSize))
	for size, media := range mediaFromPaperSize {
		m[media] = size
	}
	return m
}()

// Submit sends the document to the queue with Print-Job and returns the CUPS
// job ID. The document is streamed after the IPP message, never buffered.
func (p *Provider) Submit(ctx context.Context, queue string, doc io.Reader, opts printer.Options) (string, error) {
	req := p.newRequest(goipp.OpPrintJob)
	req.Operation.Add(goipp.MakeAttr("printer-uri", goipp.TagURI, goipp.String(p.ippURI("/printers/"+queue))))
	req.Operation.Add(goipp.MakeAttr("requesting-user-name", goipp.TagName, goipp.String(p.user)))
	req.Operation.Add(goipp.MakeAttr("job-name", goipp.TagName, goipp.String("PrintHub")))
	req.Operation.Add(goipp.MakeAttr("document-format", goipp.TagMimeType, goipp.String("application/pdf")))
	for _, attr := range jobAttributes(opts) {
		req.Job.Add(attr)
	}

	resp, err := p.do(ctx, "/printers/"+queue, req, doc)
	switch {
	case hasStatus(err, goipp.StatusErrorNotAcceptingJobs), hasStatus(err, goipp.StatusErrorNotFound):
		return "", printing.ErrPrinterUnavailable
	case err != nil:
		return "", err
	}

	id, ok := attrs(resp.Job).int("job-id")
	if !ok {
		return "", errors.New("cups: Print-Job response has no job-id")
	}
	return strconv.Itoa(id), nil
}

// jobAttributes encodes print options as IPP job template attributes.
func jobAttributes(opts printer.Options) []goipp.Attribute {
	attrs := []goipp.Attribute{
		goipp.MakeAttr("copies", goipp.TagInteger, goipp.Integer(opts.Copies)),
		goipp.MakeAttr("print-color-mode", goipp.TagKeyword, goipp.String(string(opts.ColorMode))),
	}
	if media, ok := mediaFromPaperSize[opts.PaperSize]; ok {
		attrs = append(attrs, goipp.MakeAttr("media", goipp.TagKeyword, goipp.String(media)))
	}
	orientation := orientationPortrait
	if opts.Orientation == printer.OrientationLandscape {
		orientation = orientationLandscape
	}
	attrs = append(attrs, goipp.MakeAttr("orientation-requested", goipp.TagEnum, goipp.Integer(orientation)))

	sides := sidesOneSided
	if opts.Duplex {
		sides = sidesTwoSidedLong
	}
	attrs = append(attrs, goipp.MakeAttr("sides", goipp.TagKeyword, goipp.String(sides)))

	if opts.PageRanges != "" {
		// Already validated against the printer; a parse error cannot happen here.
		if ranges, err := printer.ParsePageRanges(opts.PageRanges); err == nil && len(ranges) > 0 {
			values := make([]goipp.Value, len(ranges))
			for i, r := range ranges {
				values[i] = goipp.Range{Lower: r.First, Upper: r.Last}
			}
			attrs = append(attrs, goipp.MakeAttr("page-ranges", goipp.TagRange, values[0], values[1:]...))
		}
	}
	return attrs
}

// IPP job-state values (RFC 8011).
const (
	jobPending           = 3
	jobPendingHeld       = 4
	jobProcessing        = 5
	jobProcessingStopped = 6
	jobCanceled          = 7
	jobAborted           = 8
	jobCompleted         = 9
)

func (p *Provider) jobRequest(op goipp.Op, jobID string) (*goipp.Message, error) {
	id, err := strconv.Atoi(jobID)
	if err != nil {
		return nil, printing.ErrUnknownJob
	}
	req := p.newRequest(op)
	req.Operation.Add(goipp.MakeAttr("job-uri", goipp.TagURI, goipp.String(p.ippURI("/jobs/"+strconv.Itoa(id)))))
	req.Operation.Add(goipp.MakeAttr("requesting-user-name", goipp.TagName, goipp.String(p.user)))
	return req, nil
}

// JobStatus maps the CUPS job-state.
func (p *Provider) JobStatus(ctx context.Context, jobID string) (printing.JobStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := p.jobRequest(goipp.OpGetJobAttributes, jobID)
	if err != nil {
		return printing.JobStatus{}, err
	}
	req.Operation.Add(goipp.MakeAttr("requested-attributes", goipp.TagKeyword,
		goipp.String("job-state"), goipp.String("job-state-message")))

	resp, err := p.do(ctx, "/jobs", req, nil)
	if hasStatus(err, goipp.StatusErrorNotFound) {
		return printing.JobStatus{}, printing.ErrUnknownJob
	}
	if err != nil {
		return printing.JobStatus{}, err
	}

	a := attrs(resp.Job)
	state, _ := a.int("job-state")
	st := printing.JobStatus{Message: a.string("job-state-message")}
	switch state {
	case jobPending, jobPendingHeld:
		st.State = printing.JobPending
	case jobProcessing, jobProcessingStopped:
		st.State = printing.JobProcessing
	case jobCanceled:
		st.State = printing.JobCancelled
	case jobAborted:
		st.State = printing.JobFailed
	case jobCompleted:
		st.State = printing.JobCompleted
	default:
		return printing.JobStatus{}, fmt.Errorf("cups: unexpected job-state %d", state)
	}
	return st, nil
}

// Cancel cancels a job that has not finished yet.
func (p *Provider) Cancel(ctx context.Context, jobID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := p.jobRequest(goipp.OpCancelJob, jobID)
	if err != nil {
		return err
	}
	_, err = p.do(ctx, "/jobs", req, nil)
	switch {
	case hasStatus(err, goipp.StatusErrorNotFound):
		return printing.ErrUnknownJob
	case hasStatus(err, goipp.StatusErrorNotPossible):
		return printing.ErrJobFinished
	}
	return err
}
