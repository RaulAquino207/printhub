// Package document stores uploaded documents on disk. Uploads are untrusted:
// content is checked, size is bounded and the client's filename never becomes
// a path.
package document

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// ErrNotPDF means the content does not start with the PDF header.
	ErrNotPDF = errors.New("document is not a PDF")
	// ErrTooLarge means the content exceeds the storage's size limit.
	ErrTooLarge = errors.New("document is too large")
)

// pdfHeader is how every PDF file starts (PDF 1.x and 2.0).
var pdfHeader = []byte("%PDF-")

// validID restricts IDs to characters that are safe as file names, so an ID
// can never escape the storage directory.
var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Storage keeps documents as <dir>/<id>.pdf, readable only by the API's user.
type Storage struct {
	dir      string
	maxBytes int64
}

// NewStorage returns a Storage rooted at dir, creating it if needed.
func NewStorage(dir string, maxBytes int64) (*Storage, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("document storage: %w", err)
	}
	return &Storage{dir: dir, maxBytes: maxBytes}, nil
}

// MaxBytes is the largest document accepted.
func (s *Storage) MaxBytes() int64 {
	return s.maxBytes
}

// Path returns where the document with the given ID is stored.
func (s *Storage) Path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid document id %q", id)
	}
	return filepath.Join(s.dir, id+".pdf"), nil
}

// Save streams r into the document with the given ID and returns its size.
// It fails with ErrNotPDF or ErrTooLarge, and never leaves a partial file:
// data goes to a temporary file that is renamed into place only when complete.
func (s *Storage) Save(id string, r io.Reader) (size int64, err error) {
	final, err := s.Path(id)
	if err != nil {
		return 0, err
	}

	// Look at the first bytes without consuming them.
	br := bufio.NewReader(r)
	head, err := br.Peek(len(pdfHeader))
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("read document: %w", err)
	}
	if !bytes.Equal(head, pdfHeader) {
		return 0, ErrNotPDF
	}

	// CreateTemp uses mode 0600 and a random name in the same directory, so
	// the final rename is atomic.
	tmp, err := os.CreateTemp(s.dir, "upload-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("create temp file: %w", err)
	}
	// On any failure below, remove the temporary file. The named result err
	// lets this deferred function see the error being returned.
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	// Copy at most maxBytes+1: reading one byte past the limit is how we
	// know the content is too large without trusting any declared size.
	size, err = io.Copy(tmp, io.LimitReader(br, s.maxBytes+1))
	if err != nil {
		return 0, fmt.Errorf("write document: %w", err)
	}
	if size > s.maxBytes {
		return 0, ErrTooLarge
	}

	// Flush to disk before the rename, so a crash cannot leave a renamed but
	// empty file.
	if err = tmp.Sync(); err != nil {
		return 0, fmt.Errorf("sync document: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return 0, fmt.Errorf("close document: %w", err)
	}
	if err = os.Rename(tmp.Name(), final); err != nil {
		return 0, fmt.Errorf("store document: %w", err)
	}
	return size, nil
}

// Open returns the stored document for reading. The caller must close it.
func (s *Storage) Open(id string) (io.ReadCloser, error) {
	path, err := s.Path(id)
	if err != nil {
		return nil, err
	}
	// Not "return os.Open(path)": on error that would wrap a nil *os.File in
	// a non-nil io.ReadCloser (the typed-nil pitfall).
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// IDs lists the stored documents. Temporary files of uploads in progress are
// not included.
func (s *Storage) IDs() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	var ids []string
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".pdf")
		if ok && e.Type().IsRegular() && validID.MatchString(id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Remove deletes the document with the given ID. Removing a missing document
// is not an error.
func (s *Storage) Remove(id string) error {
	path, err := s.Path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove document: %w", err)
	}
	return nil
}
