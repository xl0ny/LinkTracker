package checker

import (
	"context"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
)

type Checker struct {
	github   *github.Client
	stackover *stackoverflow.Client
}

func New(githubClient *github.Client, stackoverClient *stackoverflow.Client) *Checker {
	return &Checker{github: githubClient, stackover: stackoverClient}
}

func (c *Checker) Check(ctx context.Context, link domain.Link) (changed bool, latest time.Time, err error) {
	u, err := url.Parse(link.URL)
	if err != nil {
		return false, time.Time{}, err
	}
	prev := link.LastUpdated
	switch {
	case u.Host == "github.com":
		return c.github.CheckUpdated(ctx, link.URL, prev)
	case u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com"):
		return c.stackover.CheckUpdated(ctx, link.URL, prev)
	default:
		return false, time.Time{}, nil
	}
}
