package download

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/t2bot/matrix-media-repo/common"
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

func TestShouldNotCacheNotYetUploaded(t *testing.T) {
	if shouldCacheDownloadError(fmt.Errorf("remote: %w", common.ErrMediaNotYetUploaded)) {
		t.Error("ErrMediaNotYetUploaded must not be cached")
	}
	if !shouldCacheDownloadError(fmt.Errorf("remote: %w", common.ErrMediaTooLarge)) {
		t.Error("ErrMediaTooLarge may be cached")
	}
}
