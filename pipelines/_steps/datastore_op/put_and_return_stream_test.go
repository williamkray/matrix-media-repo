package datastore_op

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
)

type trackedCloser struct {
	io.Reader
	closed chan struct{}
}

func (t *trackedCloser) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}
	return nil
}

func TestPutAndReturnStreamDoesNotHangWhenBufferTempFails(t *testing.T) {
	ctx := rcontext.InitialNoConfig()
	input := &trackedCloser{Reader: bytes.NewReader(make([]byte, 1<<20)), closed: make(chan struct{})}
	bufErr := errors.New("buffer failed")

	bufferTemp := func(_ config.DatastoreConfig, contents io.ReadCloser) (string, int64, io.ReadCloser, error) {
		return "", 0, nil, bufErr // fails without ever reading the pipe
	}
	uploadExecute := func(_ rcontext.RequestContext, _ string, _ string, r io.ReadCloser, _ string, _ string, _ string, _ datastores.Kind) (*database.DbMedia, error) {
		_, err := io.Copy(io.Discard, r)
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := putAndReturnStream(ctx, config.DatastoreConfig{}, "example.org", "abc", input, "text/plain", "f", datastores.RemoteMediaKind, bufferTemp, uploadExecute)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, bufErr) {
			t.Fatalf("expected buffer error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("putAndReturnStream hung after BufferTemp failure")
	}
}

func TestPutAndReturnStreamSuccess(t *testing.T) {
	ctx := rcontext.InitialNoConfig()
	payload := bytes.Repeat([]byte("x"), 1<<18)
	input := io.NopCloser(bytes.NewReader(payload))
	bufferTemp := func(_ config.DatastoreConfig, contents io.ReadCloser) (string, int64, io.ReadCloser, error) {
		b, err := io.ReadAll(contents)
		return "", int64(len(b)), io.NopCloser(bytes.NewReader(b)), err
	}
	uploadExecute := func(_ rcontext.RequestContext, _ string, _ string, r io.ReadCloser, _ string, _ string, _ string, _ datastores.Kind) (*database.DbMedia, error) {
		_, err := io.Copy(io.Discard, r)
		return &database.DbMedia{}, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		m, r, err := putAndReturnStream(ctx, config.DatastoreConfig{}, "example.org", "abc", input, "text/plain", "f", datastores.RemoteMediaKind, bufferTemp, uploadExecute)
		if err != nil || m == nil || r == nil {
			t.Errorf("unexpected result %v %v %v", m, r, err)
			return
		}
		b, _ := io.ReadAll(r)
		if !bytes.Equal(b, payload) {
			t.Errorf("payload mismatch")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hung")
	}
}
