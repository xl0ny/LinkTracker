//go:build integration

package db_test

import (
	"context"
	sqldb "database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/app"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/pgrepo"
)

var (
	_ scrapperapp.Repository = (*sqlrepo.Repository)(nil)
	_ scrapperapp.Repository = (*orm.Repository)(nil)
)

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	assert.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()
	root := projectRoot(t)
	dir := filepath.Join(root, "migrations")
	assert.DirExists(t, dir)
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)

	db, err := sqldb.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		if cerr := db.Close(); cerr != nil {
			t.Errorf("migrate helper: db close: %v", cerr)
		}
	})
	assert.Eventually(t, func() bool {
		return db.Ping() == nil
	}, 30*time.Second, 200*time.Millisecond)

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.ToSlash(abs), "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			t.Errorf("migrate close: source=%v db=%v", srcErr, dbErr)
		}
	})
	require.NoError(t, m.Up())
}

func startPostgres(t *testing.T) (ctx context.Context, dsn string, terminate func()) {
	t.Helper()
	ctx = context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("linktracker"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	dsn, err = pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dsn = strings.ReplaceAll(dsn, "[::1]", "127.0.0.1")
	dsn = strings.ReplaceAll(dsn, "@localhost:", "@127.0.0.1:")
	waitPostgresReady(t, ctx, dsn)
	terminate = func() {
		if err := pg.Terminate(ctx); err != nil {
			t.Logf("postgres container terminate: %v", err)
		}
	}
	return ctx, dsn, terminate
}

func waitPostgresReady(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		p, err := pgxpool.New(ctx, dsn)
		if err == nil {
			p.Close()
			return
		}
		last = err
		time.Sleep(150 * time.Millisecond)
	}
	require.NoError(t, last, "postgres did not accept connections")
}

func TestMigrationsUpOnCleanDB(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "migrations up on clean db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, dsn, terminate := startPostgres(t)
			defer terminate()
			runMigrations(t, dsn)
			repo, err := sqlrepo.NewRepository(ctx, dsn)
			require.NoError(t, err)
			defer repo.Close()
			require.NoError(t, repo.AddChat(ctx, 777))
		})
	}
}

func TestChatRepository_SQLAndORM(t *testing.T) {
	ctx, dsn, terminate := startPostgres(t)
	defer terminate()
	runMigrations(t, dsn)

	tests := []struct {
		name string
		new  func(context.Context, string) (scrapperapp.Repository, error)
	}{
		{
			name: "SQL",
			new: func(c context.Context, d string) (scrapperapp.Repository, error) {
				return sqlrepo.NewRepository(c, d)
			},
		},
		{
			name: "ORM",
			new: func(c context.Context, d string) (scrapperapp.Repository, error) {
				return orm.NewRepository(c, d)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := tt.new(ctx, dsn)
			require.NoError(t, err)
			defer repo.Close()
			runRepositoryScenarios(t, ctx, repo)
		})
	}
}

func runRepositoryScenarios(t *testing.T, ctx context.Context, repo scrapperapp.Repository) {
	t.Helper()
	chatID := int64(42)

	require.NoError(t, repo.AddChat(ctx, chatID))
	assert.ErrorIs(t, repo.AddChat(ctx, chatID), domain.ErrChatAlreadyExists)

	emptySubs, err := repo.ListSubscribedLinks(ctx, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, emptySubs)

	assert.ErrorIs(t, repo.AddLink(ctx, 999, "https://x.test", nil, nil), domain.ErrChatNotFound)

	link := "https://example.com/track"
	tags := []string{"go", "db"}
	filters := []string{"issue"}
	require.NoError(t, repo.AddLink(ctx, chatID, link, &tags, &filters))
	assert.ErrorIs(t, repo.AddLink(ctx, chatID, link, nil, nil), domain.ErrLinkAlreadyExists)

	links, err := repo.GetLinks(ctx, chatID, 0, 0)
	require.NoError(t, err)
	assert.Len(t, links, 1)
	assert.Equal(t, link, links[0].URL)
	assert.Equal(t, []string{"db", "go"}, links[0].Tags)
	assert.Equal(t, []string{"issue"}, links[0].Filters)

	require.NoError(t, repo.AddLink(ctx, chatID, "https://example.com/second", nil, nil))
	one, err := repo.GetLinks(ctx, chatID, 1, 0)
	require.NoError(t, err)
	assert.Len(t, one, 1)
	two, err := repo.GetLinks(ctx, chatID, 1, 1)
	require.NoError(t, err)
	assert.Len(t, two, 1)
	assert.NotEqual(t, one[0].URL, two[0].URL)

	allSubs, err := repo.ListSubscribedLinks(ctx, 0, 0)
	require.NoError(t, err)
	assert.Len(t, allSubs, 2)
	assert.Equal(t, chatID, allSubs[0].ChatID)
	assert.Equal(t, chatID, allSubs[1].ChatID)
	assert.Equal(t, link, allSubs[0].Link.URL)
	assert.Equal(t, "https://example.com/second", allSubs[1].Link.URL)

	subsPage1, err := repo.ListSubscribedLinks(ctx, 1, 0)
	require.NoError(t, err)
	assert.Len(t, subsPage1, 1)
	assert.Equal(t, link, subsPage1[0].Link.URL)

	subsPage2, err := repo.ListSubscribedLinks(ctx, 1, 1)
	require.NoError(t, err)
	assert.Len(t, subsPage2, 1)
	assert.Equal(t, "https://example.com/second", subsPage2[0].Link.URL)

	require.NoError(t, repo.AddChat(ctx, 99))
	require.NoError(t, repo.AddLink(ctx, 99, "https://ninety-nine.test", nil, nil))
	multiSubs, err := repo.ListSubscribedLinks(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, multiSubs, 3)
	assert.Equal(t, int64(99), multiSubs[2].ChatID)
	assert.Equal(t, "https://ninety-nine.test", multiSubs[2].Link.URL)
	require.NoError(t, repo.DeleteChat(ctx, 99))

	chatsAll, err := repo.GetChats(ctx, 0, 0)
	require.NoError(t, err)
	assert.Contains(t, chatsAll, chatID)

	chatsPage, err := repo.GetChats(ctx, 10, 0)
	require.NoError(t, err)
	assert.Contains(t, chatsPage, chatID)

	removed, err := repo.DeleteLink(ctx, chatID, "https://example.com/second")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/second", removed.URL)

	ts := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, repo.UpdateLinkUpdatedAt(ctx, chatID, link, ts))
	after, err := repo.GetLinks(ctx, chatID, 0, 0)
	require.NoError(t, err)
	assert.Len(t, after, 1)
	assert.True(t, after[0].LastUpdated.Equal(ts))

	tagID, err := repo.CreateTag(ctx, "standalone-tag")
	require.NoError(t, err)
	assert.Positive(t, tagID)
	_, err = repo.CreateTag(ctx, "standalone-tag")
	assert.ErrorIs(t, err, domain.ErrTagAlreadyExists)

	list, err := repo.ListTags(ctx, 50, 0)
	require.NoError(t, err)
	var found bool
	for _, x := range list {
		if x.ID == tagID && x.Value == "standalone-tag" {
			found = true
		}
	}
	assert.True(t, found, "ListTags should include created tag")

	require.NoError(t, repo.UpdateTag(ctx, tagID, "standalone-renamed"))
	assert.ErrorIs(t, repo.UpdateTag(ctx, 999999, "x"), domain.ErrTagNotFound)

	require.NoError(t, repo.DeleteTag(ctx, tagID))
	assert.ErrorIs(t, repo.DeleteTag(ctx, tagID), domain.ErrTagNotFound)

	require.NoError(t, repo.DeleteChat(ctx, chatID))
	assert.ErrorIs(t, repo.DeleteChat(ctx, chatID), domain.ErrChatNotFound)
	_, err = repo.GetLinks(ctx, chatID, 0, 0)
	assert.ErrorIs(t, err, domain.ErrChatNotFound)
}
