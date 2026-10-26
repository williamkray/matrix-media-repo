package _routers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func serveErrorResponse(t *testing.T, res interface{}) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRContextRouter(func(r *http.Request, ctx rcontext.RequestContext) interface{} { return res }, nil)
	req := httptest.NewRequest(http.MethodGet, "/_matrix/client/v1/media/download/example.org/abc", nil)
	ctx := context.WithValue(req.Context(), common.ContextLogger, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, common.ContextDomainConfig, &config.DomainRepoConfig{})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

// Regression: the per-IP download bucket returns RateLimitReached, which used to fall through to the default
// branch and be served as an HTTP 500 (with no error log), making clients retry and look like a server fault.
func TestRateLimitReachedIs429(t *testing.T) {
	if got := serveErrorResponse(t, _responses.RateLimitReached()).Code; got != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", got)
	}
}

func TestErrorStatusMapping(t *testing.T) {
	cases := map[int]*_responses.ErrorResponse{
		http.StatusServiceUnavailable:  _responses.ServiceUnavailable(),
		http.StatusNotFound:            _responses.NotFoundError(),
		http.StatusInternalServerError: _responses.InternalServerError("boom"),
	}
	for want, res := range cases {
		if got := serveErrorResponse(t, res).Code; got != want {
			t.Errorf("%s: expected %d, got %d", res.InternalCode, want, got)
		}
	}
}

func TestLogSamplerBounds(t *testing.T) {
	s := &logSampler{interval: time.Hour}
	if !s.allow() {
		t.Fatal("first event should be allowed")
	}
	for i := 0; i < 100; i++ {
		if s.allow() {
			t.Fatal("events within the interval must be suppressed")
		}
	}
	s = &logSampler{interval: time.Nanosecond}
	s.allow()
	time.Sleep(time.Millisecond)
	if !s.allow() {
		t.Fatal("event after the interval should be allowed")
	}
}
