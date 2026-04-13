package stackoverflow

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
	defaultAPIBase = "https://api.stackexchange.com/2.3"
	previewRunes   = 200
	withBody       = "withbody"
)

type Client struct {
	http    *http.Client
	apiBase string
}

func NewClient(httpClient *http.Client) *Client {
	return NewClientWithAPIBase(httpClient, defaultAPIBase)
}

func NewClientWithAPIBase(httpClient *http.Client, apiBase string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	return &Client{http: httpClient, apiBase: strings.TrimRight(apiBase, "/")}
}

type ownerObj struct {
	DisplayName string `json:"display_name"`
}

type questionItem struct {
	QuestionID       int64  `json:"question_id"`
	Title            string `json:"title"`
	LastActivityDate int64  `json:"last_activity_date"`
}

type answerItem struct {
	AnswerID     int64    `json:"answer_id"`
	Body         string   `json:"body"`
	Owner        ownerObj `json:"owner"`
	CreationDate int64    `json:"creation_date"`
	Link         string   `json:"link"`
}

type commentItem struct {
	CommentID    int64    `json:"comment_id"`
	Body         string   `json:"body"`
	Owner        ownerObj `json:"owner"`
	CreationDate int64    `json:"creation_date"`
	Link         string   `json:"link"`
}

type seWrapper[T any] struct {
	Items []T `json:"items"`
}

type soEventKind int

const (
	soKindAnswer soEventKind = iota
	soKindQuestionComment
	soKindAnswerComment
)

type soCandidate struct {
	kind    soEventKind
	at      int64
	answer  answerItem
	comment commentItem
}

// CheckQuestion ищет новый ответ, комментарий к вопросу или комментарий к ответу после since.
func (c *Client) CheckQuestion(ctx context.Context, questionURL string, since time.Time) (domain.LinkCheckOutcome, error) {
	id, err := parseQuestionID(questionURL)
	if err != nil {
		return domain.LinkCheckOutcome{}, err
	}
	idStr := strconv.FormatInt(id, 10)

	qURL := fmt.Sprintf("%s/questions/%s?site=stackoverflow&filter=%s", c.apiBase, idStr, withBody)
	var qwrap seWrapper[questionItem]
	if err := c.getJSON(ctx, qURL, &qwrap); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("question: %w", err)
	}
	if len(qwrap.Items) == 0 {
		return domain.LinkCheckOutcome{}, errors.New("question not found")
	}
	q := qwrap.Items[0]

	aURL := fmt.Sprintf("%s/questions/%s/answers?site=stackoverflow&order=desc&sort=creation&pagesize=100&filter=%s", c.apiBase, idStr, withBody)
	var awrap seWrapper[answerItem]
	if err := c.getJSON(ctx, aURL, &awrap); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("answers: %w", err)
	}

	cURL := fmt.Sprintf("%s/questions/%s/comments?site=stackoverflow&order=desc&sort=creation&pagesize=100&filter=%s", c.apiBase, idStr, withBody)
	var qComments seWrapper[commentItem]
	if err := c.getJSON(ctx, cURL, &qComments); err != nil {
		return domain.LinkCheckOutcome{}, fmt.Errorf("comments: %w", err)
	}

	answerIDs := make([]int64, 0, len(awrap.Items))
	for _, a := range awrap.Items {
		answerIDs = append(answerIDs, a.AnswerID)
	}
	aComments, err := c.fetchAnswerComments(ctx, answerIDs)
	if err != nil {
		return domain.LinkCheckOutcome{}, err
	}

	watermark := time.Unix(q.LastActivityDate, 0).UTC()
	for _, a := range awrap.Items {
		t := time.Unix(a.CreationDate, 0).UTC()
		if t.After(watermark) {
			watermark = t
		}
	}
	for _, cm := range qComments.Items {
		t := time.Unix(cm.CreationDate, 0).UTC()
		if t.After(watermark) {
			watermark = t
		}
	}
	for _, cm := range aComments {
		t := time.Unix(cm.CreationDate, 0).UTC()
		if t.After(watermark) {
			watermark = t
		}
	}

	if since.IsZero() {
		return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
	}

	sinceU := since.Unix()
	var best *soCandidate
	take := func(cand soCandidate) {
		if cand.at <= sinceU {
			return
		}
		if best == nil || cand.at > best.at {
			c := cand
			best = &c
		}
	}
	for _, a := range awrap.Items {
		take(soCandidate{kind: soKindAnswer, at: a.CreationDate, answer: a})
	}
	for _, cm := range qComments.Items {
		take(soCandidate{kind: soKindQuestionComment, at: cm.CreationDate, comment: cm})
	}
	for _, cm := range aComments {
		take(soCandidate{kind: soKindAnswerComment, at: cm.CreationDate, comment: cm})
	}
	if best == nil {
		return domain.LinkCheckOutcome{Changed: false, Latest: watermark}, nil
	}

	var desc string
	var latest time.Time
	switch best.kind {
	case soKindAnswer:
		latest = time.Unix(best.answer.CreationDate, 0).UTC()
		link := best.answer.Link
		if link == "" {
			link = questionURL
		}
		desc = formatSOUpdate("новый ответ", q.Title, best.answer.Owner.DisplayName, latest, best.answer.Body, link)
	case soKindQuestionComment:
		latest = time.Unix(best.comment.CreationDate, 0).UTC()
		link := best.comment.Link
		if link == "" {
			link = questionURL
		}
		desc = formatSOUpdate("новый комментарий к вопросу", q.Title, best.comment.Owner.DisplayName, latest, best.comment.Body, link)
	case soKindAnswerComment:
		latest = time.Unix(best.comment.CreationDate, 0).UTC()
		link := best.comment.Link
		if link == "" {
			link = questionURL
		}
		desc = formatSOUpdate("новый комментарий к ответу", q.Title, best.comment.Owner.DisplayName, latest, best.comment.Body, link)
	}
	return domain.LinkCheckOutcome{
		Changed:     true,
		Latest:      latest,
		Description: desc,
	}, nil
}

func (c *Client) fetchAnswerComments(ctx context.Context, answerIDs []int64) ([]commentItem, error) {
	if len(answerIDs) == 0 {
		return nil, nil
	}
	const batch = 100
	var out []commentItem
	for i := 0; i < len(answerIDs); i += batch {
		end := i + batch
		if end > len(answerIDs) {
			end = len(answerIDs)
		}
		parts := make([]string, 0, end-i)
		for _, id := range answerIDs[i:end] {
			parts = append(parts, strconv.FormatInt(id, 10))
		}
		u := fmt.Sprintf("%s/answers/%s/comments?site=stackoverflow&order=desc&sort=creation&pagesize=100&filter=%s",
			c.apiBase, strings.Join(parts, ";"), withBody)
		var wrap seWrapper[commentItem]
		if err := c.getJSON(ctx, u, &wrap); err != nil {
			return nil, fmt.Errorf("answer comments: %w", err)
		}
		out = append(out, wrap.Items...)
	}
	return out, nil
}

func formatSOUpdate(kind, questionTitle, author string, at time.Time, body, link string) string {
	preview := textutil.Preview(textutil.StripHTML(body), previewRunes)
	var b strings.Builder
	b.WriteString("Stack Overflow · ")
	b.WriteString(kind)
	b.WriteString("\n")
	b.WriteString("Тема: ")
	b.WriteString(questionTitle)
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
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("stackoverflow api: status=%d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

func parseQuestionID(raw string) (int64, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "stackoverflow.com" && !strings.HasSuffix(u.Host, ".stackoverflow.com") {
		return 0, errors.New("not stackoverflow url")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	const minPathParts = 2
	if len(parts) < minPathParts || parts[0] != "questions" {
		return 0, errors.New("invalid path")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, errors.New("invalid question id")
	}
	return id, nil
}

// CheckUpdated оставлен для обратной совместимости; для ДЗ используйте CheckQuestion.
func (c *Client) CheckUpdated(ctx context.Context, questionURL string) (latest time.Time, err error) {
	out, err := c.CheckQuestion(ctx, questionURL, time.Time{})
	if err != nil {
		return time.Time{}, err
	}
	return out.Latest, nil
}
