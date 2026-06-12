package prometrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type RED struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewRED(app string) *RED {
	return &RED{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
			ConstLabels: prometheus.Labels{
				labelApp: app,
			},
		}, []string{labelMethod, labelRoute, labelStatus}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
			ConstLabels: prometheus.Labels{
				labelApp: app,
			},
			Buckets: prometheus.DefBuckets,
		}, []string{labelMethod, labelRoute, labelStatus}),
	}
}

func (r *RED) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
		next.ServeHTTP(ww, req)

		route := req.URL.Path
		if rc := chi.RouteContext(req.Context()); rc != nil {
			if pattern := rc.RoutePattern(); pattern != "" {
				route = pattern
			}
		}

		status := strconv.Itoa(ww.Status())
		labels := prometheus.Labels{
			labelMethod: req.Method,
			labelRoute:  route,
			labelStatus: status,
		}
		r.Requests.With(labels).Inc()
		r.Duration.With(labels).Observe(time.Since(start).Seconds())
	})
}
