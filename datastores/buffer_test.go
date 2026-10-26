package datastores

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/t2bot/matrix-media-repo/common/config"
)

type failingReader struct{ closed bool }

func (f *failingReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }
func (f *failingReader) Close() error               { f.closed = true; return nil }

func TestBufferTempReleasesOnCopyError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tmp")
	r := &failingReader{}
	_, _, _, err := BufferTemp(config.DatastoreConfig{Type: "s3", Options: map[string]string{"tempPath": dir}}, r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !r.closed {
		t.Error("contents not closed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestBufferTempClosesContentsOnEarlyError(t *testing.T) {
	r := &failingReader{}
	_, _, _, err := BufferTemp(config.DatastoreConfig{Type: "bogus"}, r)
	if err == nil || !r.closed {
		t.Errorf("err=%v closed=%v", err, r.closed)
	}
}
