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
)

type Checker struct {
	github    *github.Client
	stackover *stackoverflow.Client
}

func New(githubClient *github.Client, stackoverClient *stackoverflow.Client) *Checker {
	return &Checker{github: githubClient, stackover: stackoverClient}
}

func (c *Checker) Check(ctx context.Context, link domain.Link) (changed bool, latest time.Time, err error) {
	u, err := url.Parse(link.URL)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("parse url: %w", err)
	}
	prev := link.LastUpdated
	switch {
	case u.Host == "github.com":
		latest, err = c.github.CheckUpdated(ctx, link.URL)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("github check: %w", err)
		}
		return latest.After(prev), latest, nil
	case u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com"):
		latest, err = c.stackover.CheckUpdated(ctx, link.URL)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("stackoverflow check: %w", err)
		}
		return latest.After(prev), latest, nil
	default:
		return false, time.Time{}, nil
	}
}
