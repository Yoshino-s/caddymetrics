package extend_metrics

import (
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpMetrics = struct {
	init             sync.Once
	requestInFlight  *prometheus.GaugeVec
	requestCount     *prometheus.CounterVec
	requestErrors    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	responseDuration *prometheus.HistogramVec
}{
	init: sync.Once{},
}

func init() {
	caddy.RegisterModule(CaddyMetrics{})
	httpcaddyfile.RegisterHandlerDirective("extend_metrics", parseCaddyfile)

	const ns, sub = "caddy", "http_extend"

	basicLabels := []string{"host"}
	httpMetrics.requestInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "requests_in_flight",
		Help:      "Number of requests currently handled by this server.",
	}, basicLabels)
	httpMetrics.requestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "request_errors_total",
		Help:      "Number of requests resulting in middleware errors.",
	}, basicLabels)
	httpMetrics.requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "requests_total",
		Help:      "Counter of HTTP(S) requests made.",
	}, basicLabels)

	// TODO: allow these to be customized in the config
	durationBuckets := prometheus.DefBuckets
	sizeBuckets := prometheus.ExponentialBuckets(256, 4, 8)

	httpLabels := []string{"host", "code", "method"}
	httpMetrics.requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "request_duration_seconds",
		Help:      "Histogram of round-trip request durations.",
		Buckets:   durationBuckets,
	}, httpLabels)
	httpMetrics.requestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "request_size_bytes",
		Help:      "Total size of the request. Includes body",
		Buckets:   sizeBuckets,
	}, httpLabels)
	httpMetrics.responseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "response_size_bytes",
		Help:      "Size of the returned response.",
		Buckets:   sizeBuckets,
	}, httpLabels)
	httpMetrics.responseDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "response_duration_seconds",
		Help:      "Histogram of times to first byte in response bodies.",
		Buckets:   durationBuckets,
	}, httpLabels)
}
