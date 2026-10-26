package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var HttpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_http_requests_total",
}, []string{"host", "action", "method"})
var InvalidHttpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_invalid_http_requests_total",
}, []string{"action", "method"})
var HttpResponses = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_http_responses_total",
}, []string{"host", "action", "method", "statusCode"})
var HttpResponseTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "media_http_response_time_seconds",
}, []string{"host", "action", "method"})
var CacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_cache_hits_total",
}, []string{"cache"})
var CacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_cache_misses_total",
}, []string{"cache"})
var ThumbnailsGenerated = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_thumbnails_generated_total",
}, []string{"width", "height", "method", "animated", "origin"})
var MediaDownloaded = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_downloaded_total",
}, []string{"origin"})
var UrlPreviewsGenerated = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_url_previews_generated_total",
}, []string{"type"})
var S3Operations = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_s3_operations_total",
}, []string{"operation"})
var MediaAgeAccessed = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: "media_age_accessed_media_seconds",
	Buckets: []float64{
		5, 15, 30, 60, 120, 150, 300, 900, 1800, 3600, 9000, 18000, 43200, 86400, 1296000, 2592000, 15811200, 31536000,
	},
})

var OpenS3Objects = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "media_s3_open_objects",
})
var StreamCopyAborted = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_stream_copy_aborted_total",
}, []string{"reason"})
var FederatedDownloadsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "media_federated_downloads_in_flight",
})
var FederatedDownloadDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "media_federated_download_seconds",
	Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 15, 20, 30, 60, 120},
}, []string{"outcome"})
var PoolWaiting = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "media_pool_waiting",
}, []string{"pool"})
var PoolOverloads = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_pool_overloads_total",
}, []string{"pool"})
var PoolPanics = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "media_pool_panics_total",
}, []string{"pool"})

func init() {
	prometheus.MustRegister(OpenS3Objects)
	prometheus.MustRegister(StreamCopyAborted)
	prometheus.MustRegister(FederatedDownloadsInFlight)
	prometheus.MustRegister(FederatedDownloadDuration)
	prometheus.MustRegister(PoolWaiting)
	prometheus.MustRegister(PoolOverloads)
	prometheus.MustRegister(PoolPanics)
	prometheus.MustRegister(HttpRequests)
	prometheus.MustRegister(InvalidHttpRequests)
	prometheus.MustRegister(HttpResponses)
	prometheus.MustRegister(HttpResponseTime)
	prometheus.MustRegister(CacheHits)
	prometheus.MustRegister(CacheMisses)
	prometheus.MustRegister(ThumbnailsGenerated)
	prometheus.MustRegister(MediaDownloaded)
	prometheus.MustRegister(UrlPreviewsGenerated)
	prometheus.MustRegister(S3Operations)
	prometheus.MustRegister(MediaAgeAccessed)
}
