package checker

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/prometrics"
)

const (
	hostGitHub        = "github.com"
	hostStackOverflow = "stackoverflow.com"
)

type Checker struct {
	github    *github.Client
	stackover *stackoverflow.Client
	metrics   *prometrics.Scrapper
}

func New(githubClient *github.Client, stackoverClient *stackoverflow.Client) *Checker {
	return &Checker{github: githubClient, stackover: stackoverClient}
}

func (c *Checker) WithMetrics(m *prometrics.Scrapper) *Checker {
	c.metrics = m
	return c
}

func (c *Checker) Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	if c.metrics != nil {
		start := time.Now()
		defer func() {
			prometrics.ObserveDuration(c.metrics.RequestDuration, prometrics.ScopeExternalSource, externalDomain(link.URL), start)
		}()
	}

	u, err := url.Parse(link.URL)
	if err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("parse url: %w", err)
	}
	prev := link.LastUpdated
	switch {
	case u.Host == hostGitHub:
		ghOut, ghErr := c.github.CheckLink(ctx, link.URL, prev)
		if ghErr != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("github check: %w", ghErr)
		}
		return ghOut, nil
	case u.Host == hostStackOverflow || strings.HasSuffix(u.Host, "."+hostStackOverflow):
		soOut, soErr := c.stackover.CheckQuestion(ctx, link.URL, prev)
		if soErr != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("stackoverflow check: %w", soErr)
		}
		return soOut, nil
	default:
		return domain.LinkCheckOutcome{}, nil
	}
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
