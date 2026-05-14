package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/textutil"
)

const (
	previewRunes            = 200
	minPathPartsGitHub      = 2
	pathPartsForIssueOrPull = 4
)

type Client struct {
	http    *http.Client
	token   string
	baseURL string
}

func NewClient(httpClient *http.Client, token string) *Client {
	return NewClientWithAPIBase(httpClient, token, "https://api.github.com")
}

func NewClientWithAPIBase(httpClient *http.Client, token, apiBase string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	apiBase = strings.TrimRight(apiBase, "/")
	return &Client{
		http:    httpClient,
		token:   token,
		baseURL: apiBase,
	}
}

type ghRef struct {
	Owner  string
	Repo   string
	IssueN int
	IsPull bool
	IsRepo bool
}

func parseGitHubRef(raw string) (ghRef, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ghRef{}, fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "github.com" && u.Host != "www.github.com" {
		return ghRef{}, errors.New("not a github url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < minPathPartsGitHub {
		return ghRef{}, fmt.Errorf("invalid repo path: %s", u.Path)
	}
	ref := ghRef{Owner: parts[0], Repo: parts[1]}
	if len(parts) >= pathPartsForIssueOrPull {
		switch parts[2] {
		case "issues":
			n, atoiErr := strconv.Atoi(parts[3])
			if atoiErr != nil {
				return ghRef{}, fmt.Errorf("invalid issue number: %s", parts[3])
			}
			ref.IssueN = n
			return ref, nil
		case "pull":
			n, atoiErr := strconv.Atoi(parts[3])
			if atoiErr != nil {
				return ghRef{}, fmt.Errorf("invalid pull number: %s", parts[3])
			}
			ref.IssueN = n
			ref.IsPull = true
			return ref, nil
		}
	}
	ref.IsRepo = true
	return ref, nil
}

type userObj struct {
	Login string `json:"login"`
}

type issueItem struct {
	HTMLURL     string    `json:"html_url"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	User        userObj   `json:"user"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest *struct {
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
}

type repoInfo struct {
	UpdatedAt time.Time `json:"updated_at"`
	PushedAt  time.Time `json:"pushed_at"`
}

type commentItem struct {
	HTMLURL   string    `json:"html_url"`
	Body      string    `json:"body"`
	User      userObj   `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) CheckLink(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	ref, err := parseGitHubRef(link.URL)
	if err != nil {
		return domain.LinkCheckOutcome{}, err
	}
	since := link.LastUpdated
	if ref.IsRepo {
		return c.checkRepo(ctx, ref, since)
	}
	return c.checkIssueOrPull(ctx, ref, since)
}

func (c *Client) CheckUpdated(ctx context.Context, repoURL string) (latest time.Time, err error) {
	out, err := c.CheckLink(ctx, domain.Link{URL: repoURL})
	if err != nil {
		return time.Time{}, fmt.Errorf("github CheckUpdated: %w", err)
	}
	return out.Latest, nil
}

func repoIssuesListURL(baseURL string, ref ghRef, since time.Time) string {
	q := url.Values{}
	q.Set("state", "all")
	q.Set("sort", "created")
	q.Set("direction", "desc")
	q.Set("per_page", "100")
	if !since.IsZero() {
		// https://docs.github.com/en/rest/issues/issues#list-repository-issues — только issues, обновлённые не раньше since
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	return fmt.Sprintf("%s/repos/%s/%s/issues?%s", baseURL, ref.Owner, ref.Repo, q.Encode())
}

func repoActivityWatermark(repo repoInfo, issues []issueItem) time.Time {
	watermark := repo.UpdatedAt
	if repo.PushedAt.After(watermark) {
		watermark = repo.PushedAt
	}
	for _, it := range issues {
		if it.CreatedAt.After(watermark) {
			watermark = it.CreatedAt
		}
		if it.UpdatedAt.After(watermark) {
			watermark = it.UpdatedAt
		}
	}
	return watermark
}

func newestIssueCreatedAfter(issues []issueItem, since time.Time) *issueItem {
	var newest *issueItem
	for i := range issues {
		it := &issues[i]
		if !it.CreatedAt.After(since) {
			continue
		}
		if newest == nil || it.CreatedAt.After(newest.CreatedAt) {
			newest = it
		}
	}
	return newest
}

func linkCheckOutcomeFromNewRepoIssue(it *issueItem) domain.LinkCheckOutcome {
	kind := "Issue"
	link := it.HTMLURL
	if it.PullRequest != nil {
		kind = "PR"
		if it.PullRequest.HTMLURL != "" {
			link = it.PullRequest.HTMLURL
		}
	}
	desc := formatGitHubUpdate(kind, it.Title, it.User.Login, it.CreatedAt, it.Body, link)
	return domain.LinkCheckOutcome{
		Changed:     true,
		Latest:      it.CreatedAt,
		Description: desc,
	}
}

func (c *Client) checkRepo(ctx context.Context, ref ghRef, since time.Time) (domain.LinkCheckOutcome, error) {
	apiURL := repoIssuesListURL(c.baseURL, ref, since)
	var list []issueItem
	if err := c.getJSON(ctx, apiURL, &list); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("list issues: %w", err)
	}
	var repo repoInfo
	repoURL := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, ref.Owner, ref.Repo)
	if err := c.getJSON(ctx, repoURL, &repo); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("repo info: %w", err)
	}
	watermark := repoActivityWatermark(repo, list)
	if since.IsZero() {
		return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
	}
	if newest := newestIssueCreatedAfter(list, since); newest != nil {
		return linkCheckOutcomeFromNewRepoIssue(newest), nil
	}
	return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
}

func (c *Client) checkIssueOrPull(ctx context.Context, ref ghRef, since time.Time) (domain.LinkCheckOutcome, error) {
	issueURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d", c.baseURL, ref.Owner, ref.Repo, ref.IssueN)
	var issue issueItem
	if err := c.getJSON(ctx, issueURL, &issue); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("get issue: %w", err)
	}

	commentsURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100&sort=created&direction=desc", c.baseURL, ref.Owner, ref.Repo, ref.IssueN)
	var comments []commentItem
	if err := c.getJSON(ctx, commentsURL, &comments); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("list comments: %w", err)
	}

	watermark := issue.UpdatedAt
	if issue.CreatedAt.After(watermark) {
		watermark = issue.CreatedAt
	}
	for _, cm := range comments {
		if cm.CreatedAt.After(watermark) {
			watermark = cm.CreatedAt
		}
	}

	if since.IsZero() {
		return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
	}

	var newest *commentItem
	for i := range comments {
		cm := &comments[i]
		if !cm.CreatedAt.After(since) {
			continue
		}
		if newest == nil || cm.CreatedAt.After(newest.CreatedAt) {
			newest = cm
		}
	}
	if newest == nil {
		return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
	}

	kind := "комментарий к Issue"
	if ref.IsPull {
		kind = "комментарий к PR"
	}
	desc := formatGitHubUpdate(kind, issue.Title, newest.User.Login, newest.CreatedAt, newest.Body, newest.HTMLURL)
	return domain.LinkCheckOutcome{
		Changed:     true,
		Latest:      newest.CreatedAt,
		Description: desc,
	}, nil
}

func formatGitHubUpdate(kind, title, author string, at time.Time, body, link string) string {
	preview := textutil.Preview(textutil.StripHTML(body), previewRunes)
	var b strings.Builder
	b.WriteString("GitHub · ")
	b.WriteString(kind)
	b.WriteString("\n")
	b.WriteString("Название: ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("Пользователь: ")
	b.WriteString(author)
	b.WriteString("\n")
	b.WriteString("Время: ")
	b.WriteString(at.UTC().Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString("Превью: ")
	b.WriteString(preview)
	b.WriteString("\n")
	b.WriteString("Ссылка: ")
	b.WriteString(link)
	return b.String()
}

func (c *Client) getJSON(ctx context.Context, apiURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found: %s", apiURL)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("github api error: status=%d", resp.StatusCode)
	}
	if decErr := json.NewDecoder(resp.Body).Decode(dst); decErr != nil {
		return fmt.Errorf("decode: %w", decErr)
	}
	return nil
}
