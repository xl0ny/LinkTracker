package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/loadtest/config"
)

const stackOverflowQuestionIDBase = 100000

func main() {
	cfgPath := flag.String("config", config.DefaultPath, "path to loadtest config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	if err = run(cfg.Seed.DSN, cfg.Seed); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run(dsn string, cfg config.Seed) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	start := time.Now()
	log.Printf("seed: chats=%d links_per_chat=%d total_links=%d", cfg.Chats, cfg.LinksPerChat, cfg.Chats*cfg.LinksPerChat)

	if _, err = pool.Exec(ctx, `TRUNCATE link_filter, link_tag, filter, tag, subscriptions, links, chats RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	chatIDs, err := insertChats(ctx, pool, cfg.Chats, cfg.ChatIDBase)
	if err != nil {
		return err
	}
	log.Printf("seed: chats inserted in %s", time.Since(start))

	urlToID, err := insertLinks(ctx, pool, cfg.LinksPerChat)
	if err != nil {
		return err
	}
	log.Printf("seed: links inserted in %s", time.Since(start))

	inserted, err := insertSubscriptions(ctx, pool, chatIDs, urlToID, cfg.Batch)
	if err != nil {
		return err
	}
	log.Printf("seed: done in %s, total subscriptions=%d", time.Since(start), inserted)
	return nil
}

func insertChats(ctx context.Context, pool *pgxpool.Pool, n int, chatIDBase int64) ([]int64, error) {
	ids := make([]int64, 0, n)
	for i := range n {
		var id int64
		tgID := chatIDBase + int64(i)
		if err := pool.QueryRow(ctx, `INSERT INTO chats (telegram_id) VALUES ($1) RETURNING id`, tgID).Scan(&id); err != nil {
			return nil, fmt.Errorf("insert chat %d: %w", tgID, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func insertLinks(ctx context.Context, pool *pgxpool.Pool, n int) (map[string]int64, error) {
	urls := make([]string, 0, n)
	for i := range n {
		if i%2 == 0 {
			urls = append(urls, fmt.Sprintf("https://github.com/link-tracker/demo-%d", i))
			continue
		}
		urls = append(urls, fmt.Sprintf("https://stackoverflow.com/questions/%d/link-tracker-demo", stackOverflowQuestionIDBase+i))
	}
	urlToID := make(map[string]int64, n)
	for _, u := range urls {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO links (url) VALUES ($1) ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url RETURNING id`, u).Scan(&id); err != nil {
			return nil, fmt.Errorf("insert link %s: %w", u, err)
		}
		urlToID[u] = id
	}
	return urlToID, nil
}

func insertSubscriptions(ctx context.Context, pool *pgxpool.Pool, chatIDs []int64, urlToID map[string]int64, batch int) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted := 0
	for _, chatID := range chatIDs {
		for url, linkID := range urlToID {
			var subID int64
			if err = tx.QueryRow(ctx, `
				INSERT INTO subscriptions (chat_id, link_id)
				VALUES ($1, $2)
				ON CONFLICT (chat_id, link_id) DO UPDATE SET chat_id = EXCLUDED.chat_id
				RETURNING id`, chatID, linkID).Scan(&subID); err != nil {
				return inserted, fmt.Errorf("insert subscription: %w", err)
			}
			if err = insertDemoTags(ctx, tx, subID, url); err != nil {
				return inserted, err
			}
			inserted++
			if inserted%batch == 0 {
				log.Printf("seed: subscriptions %d", inserted)
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

func insertDemoTags(ctx context.Context, tx pgx.Tx, subID int64, url string) error {
	tags := []string{"loadtest"}
	switch {
	case strings.Contains(url, "github.com"):
		tags = append(tags, "github")
	case strings.Contains(url, "stackoverflow.com"):
		tags = append(tags, "stackoverflow")
	}
	for _, value := range tags {
		var tagID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO tag (value)
			VALUES ($1)
			ON CONFLICT (value) DO UPDATE SET value = tag.value
			RETURNING id`, value).Scan(&tagID); err != nil {
			return fmt.Errorf("insert tag %q: %w", value, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO link_tag (subscription_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, subID, tagID); err != nil {
			return fmt.Errorf("insert link tag %q: %w", value, err)
		}
	}
	return nil
}
