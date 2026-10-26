package upload

import (
	"bytes"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

// The spam goroutine must exit even when nobody ever receives its result (e.g. Execute returned early on an
// upload error).
func TestCheckSpamAsyncGoroutineExitsWithoutReceiver(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		_ = CheckSpamAsync(rcontext.InitialNoConfig(), io.NopCloser(bytes.NewReader([]byte("data"))), FileMetadata{})
		// result channel deliberately never read
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked: before=%d after=%d", before, runtime.NumGoroutine())
}

func TestCheckSpamAsyncDeliversResult(t *testing.T) {
	ch := CheckSpamAsync(rcontext.InitialNoConfig(), bytes.NewReader([]byte("data")), FileMetadata{})
	select {
	case res := <-ch:
		if res.Err != nil || res.IsSpam {
			t.Fatalf("unexpected %+v", res)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no result")
	}
}
