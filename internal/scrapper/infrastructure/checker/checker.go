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
	switch {
	// GitHub: клиент и parseGitHubRef рассчитаны на хост github.com (репозитории, issues, PR).
	// Не используем HasSuffix(".github.com"): gist.github.com, raw.githubusercontent.com и т.п. —
	// другие API; их нельзя обрабатывать тем же CheckLink без отдельной логики.
	// www — частый вариант записи той же страницы.
	case u.Host == "github.com" || u.Host == "www.github.com":
		ghOut, ghErr := c.github.CheckLink(ctx, link)
		if ghErr != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("github check: %w", ghErr)
		}
		return ghOut, nil
	// Stack Exchange: региональные и meta-сайты (ru.*, meta.*, …) с тем же путём /questions/….
	case u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com"):
		soOut, soErr := c.stackover.CheckQuestion(ctx, link)
		if soErr != nil {
			return domain.LinkCheckOutcome{}, fmt.Errorf("stackoverflow check: %w", soErr)
		}
		return soOut, nil
	default:
		return domain.LinkCheckOutcome{}, nil
	}
}
