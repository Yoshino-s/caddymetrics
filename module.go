package extend_metrics

import (
	"errors"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	ServerCtxKey caddy.CtxKey = "server"
)

func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s += len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}

// Gizmo is an example; put your own type here.
type CaddyMetrics struct {
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (CaddyMetrics) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.extend_metrics",
		New: func() caddy.Module { return new(CaddyMetrics) },
	}
}

func (c *CaddyMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	labels := prometheus.Labels{"host": r.Host}
	method := SanitizeMethod(r.Method)
	// the "code" value is set later, but initialized here to eliminate the possibility
	// of a panic
	statusLabels := prometheus.Labels{"host": r.Host, "method": method, "code": "0"}

	inFlight := httpMetrics.requestInFlight.With(labels)
	inFlight.Inc()
	defer inFlight.Dec()

	start := time.Now()

	// This is a _bit_ of a hack - it depends on the ShouldBufferFunc always
	// being called when the headers are written.
	// Effectively the same behaviour as promhttp.InstrumentHandlerTimeToWriteHeader.
	writeHeaderRecorder := caddyhttp.ShouldBufferFunc(func(status int, header http.Header) bool {
		statusLabels["code"] = SanitizeCode(status)
		ttfb := time.Since(start).Seconds()
		httpMetrics.responseDuration.With(statusLabels).Observe(ttfb)
		return false
	})
	wrec := caddyhttp.NewResponseRecorder(w, nil, writeHeaderRecorder)
	err := next.ServeHTTP(wrec, r)
	dur := time.Since(start).Seconds()
	httpMetrics.requestCount.With(labels).Inc()

	observeRequest := func(status int) {
		// If the code hasn't been set yet, and we didn't encounter an error, we're
		// probably falling through with an empty handler.
		if statusLabels["code"] == "" {
			// we still sanitize it, even though it's likely to be 0. A 200 is
			// returned on fallthrough so we want to reflect that.
			statusLabels["code"] = SanitizeCode(status)
		}

		httpMetrics.requestDuration.With(statusLabels).Observe(dur)
		httpMetrics.requestSize.With(statusLabels).Observe(float64(computeApproximateRequestSize(r)))
		httpMetrics.responseSize.With(statusLabels).Observe(float64(wrec.Size()))
	}

	if err != nil {
		var handlerErr caddyhttp.HandlerError
		if errors.As(err, &handlerErr) {
			observeRequest(handlerErr.StatusCode)
		}

		httpMetrics.requestErrors.With(labels).Inc()

		return err
	}

	observeRequest(wrec.Status())

	return nil
}

func (c *CaddyMetrics) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.NextArg()
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var metrics = new(CaddyMetrics)
	err := metrics.UnmarshalCaddyfile(h.Dispenser)
	return metrics, err
}

var (
	_ caddyhttp.MiddlewareHandler = (*CaddyMetrics)(nil)
	_ caddyfile.Unmarshaler       = (*CaddyMetrics)(nil)
)
