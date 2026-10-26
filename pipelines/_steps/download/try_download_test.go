package download

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestShouldCacheDownloadError(t *testing.T) {
	if shouldCacheDownloadError(fmt.Errorf("wrapped: %w", context.Canceled)) {
		t.Error("context.Canceled must not be cached")
	}
	if !shouldCacheDownloadError(context.DeadlineExceeded) {
		t.Error("deadline exceeded (remote timeout) must be cached")
	}
	if !shouldCacheDownloadError(errors.New("response not ok: 500")) {
		t.Error("ordinary errors must be cached")
	}
}
