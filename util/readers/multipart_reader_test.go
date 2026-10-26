package readers

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type trackedPart struct {
	r      io.Reader
	closes atomic.Int32
	closed chan struct{}
}

func newTrackedPart(r io.Reader) *trackedPart {
	return &trackedPart{r: r, closed: make(chan struct{}, 8)}
}

func (p *trackedPart) Read(b []byte) (int, error) { return p.r.Read(b) }
func (p *trackedPart) Close() error {
	p.closes.Add(1)
	p.closed <- struct{}{}
	return nil
}

func waitClosed(t *testing.T, name string, p *trackedPart) {
	t.Helper()
	select {
	case <-p.closed:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s was never closed", name)
	}
	time.Sleep(20 * time.Millisecond)
	assert.EqualValues(t, 1, p.closes.Load(), name)
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestMultipartClosesAllPartsOnCopyError(t *testing.T) {
	p1 := newTrackedPart(strings.NewReader("one"))
	p2 := newTrackedPart(errReader{errors.New("boom")})
	p3 := newTrackedPart(strings.NewReader("three"))
	r := NewMultipartReader("bnd",
		&MultipartPart{Reader: p1}, &MultipartPart{Reader: p2}, &MultipartPart{Reader: p3})
	_, err := io.ReadAll(r)
	assert.EqualError(t, err, "boom")
	waitClosed(t, "p1", p1)
	waitClosed(t, "p2", p2)
	waitClosed(t, "p3", p3)
	_ = r.Close()
}

func TestMultipartClosesUnreadPartsWhenConsumerCloses(t *testing.T) {
	p1 := newTrackedPart(strings.NewReader(strings.Repeat("x", 1<<20)))
	p2 := newTrackedPart(strings.NewReader("two"))
	r := NewMultipartReader("bnd", &MultipartPart{Reader: p1}, &MultipartPart{Reader: p2})
	buf := make([]byte, 16)
	_, err := io.ReadFull(r, buf)
	assert.NoError(t, err)
	assert.NoError(t, r.Close())
	waitClosed(t, "p1", p1)
	waitClosed(t, "p2", p2)
}

func TestMultipartLocationClosesOriginal(t *testing.T) {
	p1 := newTrackedPart(strings.NewReader("one"))
	r := NewMultipartReader("bnd", &MultipartPart{Reader: p1, Location: "https://example.org/x"})
	out, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.True(t, bytes.Contains(out, []byte("Location: https://example.org/x")))
	waitClosed(t, "p1", p1)
}

func TestMultipartNilReaderDoesNotPanic(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		r := NewMultipartReader("bnd", &MultipartPart{ContentType: "text/plain"})
		_, err := io.ReadAll(r)
		done <- err
	}()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}
