package document

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStorage(t *testing.T, maxBytes int64) (*Storage, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "documents")
	s, err := NewStorage(dir, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// files lists the directory, so tests can assert that nothing was left behind.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSaveStoresPDF(t *testing.T) {
	s, dir := newTestStorage(t, 1024)
	content := []byte("%PDF-1.7\nhello\n%%EOF")

	size, err := s.Save("job1", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}

	path := filepath.Join(dir, "job1.pdf")
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("stored content = %q, %v; want the original", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 600", perm)
	}
}

func TestSaveRejectsAndCleansUp(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    error
	}{
		{name: "not a pdf", content: "<html>hello</html>", want: ErrNotPDF},
		{name: "empty", content: "", want: ErrNotPDF},
		{name: "header only partially", content: "%PD", want: ErrNotPDF},
		{name: "too large", content: "%PDF-" + strings.Repeat("x", 20), want: ErrTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, dir := newTestStorage(t, 16)

			_, err := s.Save("job1", strings.NewReader(tt.content))

			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if left := files(t, dir); len(left) != 0 {
				t.Errorf("files left behind: %v", left)
			}
		})
	}
}

func TestSaveAcceptsExactLimit(t *testing.T) {
	s, _ := newTestStorage(t, 16)
	content := "%PDF-" + strings.Repeat("x", 11) // exactly 16 bytes

	if _, err := s.Save("job1", strings.NewReader(content)); err != nil {
		t.Fatalf("Save at the limit: %v", err)
	}
}

func TestIDsCannotEscapeTheDirectory(t *testing.T) {
	s, _ := newTestStorage(t, 1024)

	for _, id := range []string{"../evil", "a/b", "", ".", "x.pdf", strings.Repeat("a", 65)} {
		if _, err := s.Save(id, strings.NewReader("%PDF-1.7")); err == nil {
			t.Errorf("Save(%q) succeeded, want an invalid id error", id)
		}
	}
}

func TestRemove(t *testing.T) {
	s, dir := newTestStorage(t, 1024)
	if _, err := s.Save("job1", strings.NewReader("%PDF-1.7")); err != nil {
		t.Fatal(err)
	}

	if err := s.Remove("job1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if left := files(t, dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
	if err := s.Remove("job1"); err != nil {
		t.Errorf("Remove of a missing document = %v, want nil", err)
	}
}
