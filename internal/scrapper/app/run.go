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
	scrapperapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/swagger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/handler"
)

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repo := db.NewRepository()
	usecase := application.NewChatUC(repo)
	h := handler.NewHandler(usecase)
	r := chi.NewRouter()

	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		if _, err := w.Write(scrapperapi.ContractYAML); err != nil {
			slog.Error("swagger yaml write error", slog.String("error", err.Error()), slog.String("event", "swagger"))
		}
	})
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte(swagger.SwaggerHTML)); err != nil {
			slog.Error("swagger html write error", slog.String("error", err.Error()), slog.String("event", "swagger"))
		}
	})

	api.HandlerFromMux(h, r)

	if cfg.BotURL != "" {
		botAPI, errBot := botclient.NewClientWithResponses(cfg.BotURL)
		if errBot != nil {
			slog.Warn("bot client disabled", slog.String("error", errBot.Error()), slog.String("event", "bot_client"))
		} else {
			notifier := botclient.NewNotifier(botAPI)
			gh := github.NewClient(nil, os.Getenv("GITHUB_TOKEN"))
			so := stackoverflow.NewClient(nil)
			lc := checker.New(gh, so)
			sch := application.NewScheduler(repo, lc, notifier)
			go sch.Run(ctx)
		}
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	go func() {
		if errSrv := srv.ListenAndServe(); errSrv != nil && errSrv != http.ErrServerClosed {
			slog.Error("http server", slog.String("error", errSrv.Error()), slog.String("event", "http_server"))
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
