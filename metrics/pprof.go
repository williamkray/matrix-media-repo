package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/http/pprof" // note: its init() registers on http.DefaultServeMux, which no server here uses. Never pass a nil Handler to a server.
	"os"

	"github.com/sirupsen/logrus"
)

// bindPprof exposes the standard pprof handlers on the (non-public) metrics listener, if MEDIA_PPROF_SECRET_KEY
// is set. Requests must send `Authorization: Bearer <secret>`.
func bindPprof(mux *http.ServeMux) {
	secret := os.Getenv("MEDIA_PPROF_SECRET_KEY")
	if secret == "" {
		return
	}
	if len(secret) < 16 {
		logrus.Error("MEDIA_PPROF_SECRET_KEY is too short (min 16 chars); pprof disabled")
		return
	}
	logrus.Warn("Enabling pprof/debug http endpoints on the metrics listener")
	mux.Handle("/debug/pprof/", requirePprofSecret(secret, pprof.Index)) // also serves goroutine, heap, etc
	mux.Handle("/debug/pprof/cmdline", requirePprofSecret(secret, pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", requirePprofSecret(secret, pprof.Profile))
	mux.Handle("/debug/pprof/symbol", requirePprofSecret(secret, pprof.Symbol))
	mux.Handle("/debug/pprof/trace", requirePprofSecret(secret, pprof.Trace))
}

func requirePprofSecret(secret string, next http.HandlerFunc) http.Handler {
	want := sha256.Sum256([]byte("Bearer " + secret)) // hash so the comparison length never depends on the input
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(&map[string]bool{"success": false})
			return
		}
		next(w, r)
	})
}
