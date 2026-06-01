package checker

import (
	"context"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

type instrumented struct {
	inner *Checker
	m     *metrics.Scrapper
}

func WithMetrics(c *Checker, m *metrics.Scrapper) *instrumented {
	return &instrumented{inner: c, m: m}
}

func (c *instrumented) Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	scopeType := externalDomain(link.URL)
	start := time.Now()
	out, err := c.inner.Check(ctx, link)
	metrics.ObserveDuration(c.m.RequestDuration, metrics.ScopeExternalSource, scopeType, start)
	return out, err
}

func externalDomain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unknown"
	}
	if u.Host == "github.com" {
		return "github.com"
	}
	if u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com") {
		return "stackoverflow.com"
	}
	return u.Host
}
