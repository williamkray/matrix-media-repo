package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var poolGaugesMu sync.Mutex
var poolGauges = map[string][]prometheus.Collector{}

// RegisterPoolGauges exposes media_pool_running and media_pool_capacity for the named pool, backed by the given
// callbacks. Calling it again for the same pool replaces the earlier registration (safe for reloads/tests).
func RegisterPoolGauges(pool string, running func() float64, capacity func() float64) {
	poolGaugesMu.Lock()
	defer poolGaugesMu.Unlock()

	for _, c := range poolGauges[pool] {
		prometheus.Unregister(c)
	}

	labels := prometheus.Labels{"pool": pool}
	r := prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "media_pool_running", ConstLabels: labels}, running)
	c := prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "media_pool_capacity", ConstLabels: labels}, capacity)
	prometheus.MustRegister(r)
	prometheus.MustRegister(c)
	poolGauges[pool] = []prometheus.Collector{r, c}
}
