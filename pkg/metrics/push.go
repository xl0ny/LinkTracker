package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus/push"
)

type PushConfig struct {
	Enabled  bool
	URL      string
	Job      string
	Interval time.Duration
}

func StartPusher(ctx context.Context, reg *Registry, cfg PushConfig) {
	if !cfg.Enabled || cfg.URL == "" || cfg.Job == "" {
		return
	}
	go func() {
		p := push.New(cfg.URL, cfg.Job).Gatherer(reg.Registry)
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			if err := p.Push(); err != nil {
				slog.Warn("metrics pushgateway", slog.String("job", cfg.Job), slog.String("error", err.Error()))
			}
			select {
			case <-ctx.Done():
				if err := p.Push(); err != nil {
					slog.Warn("metrics pushgateway final", slog.String("error", fmt.Sprintf("%v", err)))
				}
				return
			case <-ticker.C:
			}
		}
	}()
	slog.Info("metrics pushgateway started", slog.String("job", cfg.Job), slog.String("url", cfg.URL))
}
