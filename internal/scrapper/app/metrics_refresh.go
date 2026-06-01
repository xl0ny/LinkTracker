package app

import (
	"context"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/metricsrepo"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

const defaultLinksRefreshInterval = 30 * time.Second

func refreshLinksOnTrack(ctx context.Context, repo *metricsrepo.Repository, m *metrics.Scrapper, interval time.Duration) {
	if interval <= 0 {
		interval = defaultLinksRefreshInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	apply := func() {
		counts, err := repo.CountTrackedLinksBySource(ctx)
		if err != nil {
			slog.Warn("scrapper metrics: links_on_track refresh", slog.String("error", err.Error()))
			return
		}
		for _, src := range []string{"github", "stackoverflow"} {
			m.LinksOnTrack.WithLabelValues(src).Set(float64(counts[src]))
		}
	}

	apply()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			apply()
		}
	}
}
