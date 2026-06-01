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

const (
	hostGitHub        = "github.com"
	hostStackOverflow = "stackoverflow.com"
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
