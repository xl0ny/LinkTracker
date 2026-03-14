package main

import (
	"context"
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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/handler"
)

const swaggerHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.29.0/swagger-ui.min.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.29.0/swagger-ui-bundle.min.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.yaml',
        dom_id: '#swagger-ui',
      });
    };
  </script>
</body>
</html>
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repo := db.NewRepository()
	usecase := application.NewChatUC(repo)
	h := handler.NewHandler(usecase)
	r := chi.NewRouter()

	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write(scrapperapi.ContractYAML)
	})
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerHTML))
	})

	api.HandlerFromMux(h, r)

	if cfg.BotURL != "" {
		botAPI, errBot := botclient.NewClientWithResponses(cfg.BotURL)
		if errBot != nil {
			slog.Warn("bot client disabled", slog.String("error", errBot.Error()))
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
			slog.Error("http server", slog.String("error", errSrv.Error()))
		}
	}()

	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
}
