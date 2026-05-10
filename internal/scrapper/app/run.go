package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	contracts "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/pgrepo"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/swagger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/handler"
)

type Repository interface {
	application.SchedulerLinks
	application.ChatRepository
	application.LinkRepository
	application.TagRepository
	Close()
}

var (
	_ Repository = (*pgrepo.Repository)(nil)
	_ Repository = (*orm.Repository)(nil)
)

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repo, err := newChatRepository(ctx, cfg)
	if err != nil {
		return err
	}
	defer repo.Close()

	usecase := application.NewChatUC(repo, repo)
	h := handler.NewHandler(usecase)
	r := chi.NewRouter()

	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		if _, werr := w.Write(contracts.Scrapper); werr != nil {
			slog.Error("scrapper run: swagger yaml write error", slog.String("error", werr.Error()))
		}
	})
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, werr := w.Write([]byte(swagger.SwaggerHTML)); werr != nil {
			slog.Error("scrapper run: swagger html write error", slog.String("error", werr.Error()))
		}
	})

	api.HandlerFromMux(h, r)

	botAPI, err := botclient.NewClientWithResponses(cfg.BotURL)
	if err != nil {
		return fmt.Errorf("bot client init: %w", err)
	}
	notifier := botclient.NewNotifier(botAPI)
	gh := github.NewClient(nil, os.Getenv("GITHUB_TOKEN"))
	so := stackoverflow.NewClient(nil)
	lc := checker.New(gh, so)
	sch := application.NewScheduler(
		repo,
		lc,
		notifier,
		cfg.Batch.Size,
		cfg.Scheduler.Workers,
		cfg.SchedulerInterval(),
	)
	go sch.Run(ctx)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		if errSrv := srv.ListenAndServe(); errSrv != nil && errSrv != http.ErrServerClosed {
			slog.Error("scrapper run: http server error", slog.String("error", errSrv.Error()))
		}
	}()
	swaggerUI := fmt.Sprintf("http://127.0.0.1:%s/swagger", cfg.Port)
	slog.Info("scrapper run: http server started",
		slog.String("port", cfg.Port),
		slog.String("swagger_ui", swaggerUI))

	<-ctx.Done()
	if shutdownErr := srv.Shutdown(context.Background()); shutdownErr != nil {
		return fmt.Errorf("shutdown: %w", shutdownErr)

	}
	return nil
}

func newChatRepository(ctx context.Context, cfg *config.Config) (Repository, error) {
	mode := cfg.AccessType
	if mode == "" {
		mode = config.AccessSQL
	}
	dsn := cfg.PostgresDSN()
	switch mode {
	case config.AccessSQL:
		repo, sqlErr := pgrepo.NewRepository(ctx, dsn)
		if sqlErr != nil {
			return nil, fmt.Errorf("scrapper: pgrepo repository: %w", sqlErr)
		}
		return repo, nil
	case config.AccessORM:
		repo, ormErr := orm.NewRepository(ctx, dsn)
		if ormErr != nil {
			return nil, fmt.Errorf("scrapper: orm repository: %w", ormErr)
		}
		return repo, nil
	default:
		return nil, fmt.Errorf("scrapper: invalid access_type %q (use SQL or ORM)", mode)
	}
}
