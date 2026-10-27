package matrix

import (
	"errors"
	"net/http"
	"testing"

	"github.com/t2bot/matrix-media-repo/common"
)

func TestFederatedStatusError(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{http.StatusBadGateway, `{"errcode":"M_TOO_LARGE","error":"x"}`, common.ErrMediaTooLarge},
		{http.StatusGatewayTimeout, `{"errcode":"M_NOT_YET_UPLOADED"}`, common.ErrMediaNotYetUploaded},
		{http.StatusBadGateway, `{"errcode":"M_UNKNOWN"}`, nil},
		{http.StatusBadGateway, `not json`, nil},
		{http.StatusBadGateway, ``, nil},
		{http.StatusInternalServerError, `{"errcode":"M_TOO_LARGE"}`, nil},
		{http.StatusGatewayTimeout, `{"errcode":"M_TOO_LARGE"}`, nil},
	}
	for _, c := range cases {
		err := federatedStatusError(c.status, []byte(c.body))
		if err == nil {
			t.Fatalf("%d %q: expected error", c.status, c.body)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%d %q: expected %v, got %v", c.status, c.body, c.want, err)
		}
		if c.want == nil && (errors.Is(err, common.ErrMediaTooLarge) || errors.Is(err, common.ErrMediaNotYetUploaded)) {
			t.Errorf("%d %q: unexpected sentinel: %v", c.status, c.body, err)
		}
	}
}
