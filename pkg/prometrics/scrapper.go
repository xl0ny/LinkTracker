package prometrics

import (
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	ScopeDatabase       = "database"
	ScopeExternalSource = "external_source"
	ScopeKafka          = "kafka"
	ScopeLLMAgent       = "llm_agent"
)

type Scrapper struct {
	LinksOnTrack    *prometheus.GaugeVec
	RequestDuration *prometheus.HistogramVec
	APIRequests     *prometheus.CounterVec
	RED             *RED
}

func NewScrapper(reg *Registry) *Scrapper {
	labels := prometheus.Labels{"app": reg.App}
	s := &Scrapper{
		LinksOnTrack: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "links_on_track_total",
			Help:        "Links in DB on monitoring",
			ConstLabels: labels,
		}, []string{"tracked_source"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "request_duration_ms_total",
			Help:        "Operation duration in milliseconds",
			ConstLabels: labels,
			Buckets:     durationBuckets,
		}, []string{"scope", "scope_type"}),
		APIRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "api_requests_total",
			Help:        "Incoming API requests",
			ConstLabels: labels,
		}, []string{"source"}),
		RED: NewRED(reg.App),
	}
	reg.MustRegister(
		s.LinksOnTrack,
		s.RequestDuration,
		s.APIRequests,
		s.RED.Requests,
		s.RED.Duration,
	)
	return s
}

func (s *Scrapper) APIRequestsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source := r.Header.Get("X-Metrics-Source")
		if source == "" {
			source = "http"
		}
		s.APIRequests.WithLabelValues(source).Inc()
		next.ServeHTTP(w, r)
	})
}

const llmAgentScopeType = "huggingface"

func IsLLMPipelineTopic(topic string) bool {
	t := strings.ToLower(strings.TrimSpace(topic))
	return strings.Contains(t, "raw-updates") && !strings.Contains(t, "dlq")
}

func (s *Scrapper) ObserveKafkaWrite(topic string, start time.Time) {
	if s == nil {
		return
	}
	ObserveDuration(s.RequestDuration, ScopeKafka, topic, start)
	if IsLLMPipelineTopic(topic) {
		ObserveDuration(s.RequestDuration, ScopeLLMAgent, llmAgentScopeType, start)
	}
}
