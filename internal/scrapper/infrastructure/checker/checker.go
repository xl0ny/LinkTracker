package checker

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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

func (c *Checker) Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	u, err := url.Parse(link.URL)
	if err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("parse url: %w", err)
	}
	prev := link.LastUpdated
	switch {
	case u.Host == "github.com":
		out, err := c.github.CheckLink(ctx, link.URL, prev)
		if err != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("github check: %w", err)
		}
		return out, nil
	case u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com"):
		out, err := c.stackover.CheckQuestion(ctx, link.URL, prev)
		if err != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("stackoverflow check: %w", err)
		}
		return out, nil
	default:
		return domain.LinkCheckOutcome{}, nil
	}
}
