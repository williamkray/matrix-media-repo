package matrix

import (
	"net/http"
	"testing"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func TestNewHttpClientDoesNotMutateSharedDeniedSlice(t *testing.T) {
	backing := make([]string, 1, 10)
	backing[0] = "10.0.0.0/8"
	cfg := &HttpClientConfig{DeniedCIDRs: backing, Timeout: time.Second}
	NewHttpClient(rcontext.InitialNoConfig(), cfg)
	if got := backing[:2][1]; got != "" {
		t.Fatalf("shared backing array was written to: %q", got)
	}
	if len(cfg.DeniedCIDRs) != 3 {
		t.Fatalf("expected 3 denied cidrs, got %v", cfg.DeniedCIDRs)
	}
}

func TestNewHttpClientRedirectWithoutTLSConfig(t *testing.T) {
	c := NewHttpClient(rcontext.InitialNoConfig(), &HttpClientConfig{Timeout: time.Second, FollowRedirects: true})
	req, _ := http.NewRequest(http.MethodGet, "https://example.org/", nil)
	// no TLSServerName => nil TLSClientConfig; must not panic
	if err := c.CheckRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}
