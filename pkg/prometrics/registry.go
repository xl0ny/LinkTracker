package prometrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	App      string
	Registry *prometheus.Registry
}

func New(app string) *Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return &Registry{App: app, Registry: reg}
}

func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{})
}

func (r *Registry) MustRegister(cs ...prometheus.Collector) {
	r.Registry.MustRegister(cs...)
}
