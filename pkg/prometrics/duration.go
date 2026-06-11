package prometrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var durationMillisecondsBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000}

func NewDurationHistogram(name, help string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: durationMillisecondsBuckets,
	}, []string{"scope", "scope_type"})
}

func ObserveDuration(h *prometheus.HistogramVec, scope, scopeType string, start time.Time) {
	h.WithLabelValues(scope, scopeType).Observe(float64(time.Since(start).Milliseconds()))
}

func ObserveOperationDuration(h *prometheus.HistogramVec, scope, scopeType string, operation func() error) error {
	start := time.Now()
	err := operation()
	ObserveDuration(h, scope, scopeType, start)
	return err
}
