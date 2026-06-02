package checker

import (
	"context"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/prometrics"
)

type LinkChecker interface {
	Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error)
}

type instrumented struct {
	inner *Checker
	m     *prometrics.Scrapper
}

func WithMetrics(c *Checker, m *prometrics.Scrapper) LinkChecker {
	if c == nil {
		return nil
	}
	return &instrumented{inner: c, m: m}
}

func (c *instrumented) Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	scopeType := externalDomain(link.URL)
	start := time.Now()
	out, err := c.inner.Check(ctx, link)
	prometrics.ObserveDuration(c.m.RequestDuration, prometrics.ScopeExternalSource, scopeType, start)
	return out, err
}

func externalDomain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unknown"
	}
	if u.Host == hostGitHub {
		return hostGitHub
	}
	if u.Host == hostStackOverflow || strings.HasSuffix(u.Host, "."+hostStackOverflow) {
		return hostStackOverflow
	}
	return u.Host
}
