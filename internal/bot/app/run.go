package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/go-chi/chi/v5"
	contracts "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/scrapperclient"
	botswagger "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/swagger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	bothttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http"
	botapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http/api"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("run: bot initialization error: %w", err)
	}

	tracker, err := scrapperclient.NewLinkTracker(cfg.ScrapperURL)
	if err != nil {
		slog.Warn("scrapper client disabled", slog.String("error", err.Error()), slog.String("event", "scrapper_client"))
		tracker = application.NewNoopTracker()
	}

	stateStore := application.NewTrackStateStore()
	trackCmd := command.NewTrack(tracker, stateStore)
	commands := command.Commands(tracker, trackCmd, tracker, tracker)
	bot.SetMenuCommands(commands)

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands, trackCmd, stateStore)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			application.Worker(actions, dispatcher, bot)
		})
	}

	srv, err := startHTTPServer(ctx, cfg, bot)
	if err != nil {
		return err
	}

	<-ctx.Done()

	shutdownErr := srv.Shutdown(context.Background())
	if shutdownErr != nil {
		slog.Error("run: server graceful shutdown error", slog.String("error", shutdownErr.Error()))
	}
	slog.Info("run: shutting down")
	wg.Wait()
	return nil
}

func startHTTPServer(ctx context.Context, cfg *config.Config, sender bothttp.MessageSender) (*http.Server, error) {
	r := chi.NewRouter()
	mountSwagger(r)

	updatesHandler := bothttp.NewHandler(sender)
	botapi.HandlerFromMux(updatesHandler, r)

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":"+cfg.BotPort)
	if err != nil {
		slog.Error("run: http server bind error", slog.String("error", err.Error()), slog.String("port", cfg.BotPort))
		return nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: r}
	go func() {
		if errSrv := srv.Serve(ln); errSrv != nil && errSrv != http.ErrServerClosed {
			slog.Error("run: http server serve error", slog.String("error", errSrv.Error()))
		}
	}()
	slog.Info("run: http server started",
		slog.String("port", cfg.BotPort),
		slog.String("swagger_ui", fmt.Sprintf("http://127.0.0.1:%s/swagger", cfg.BotPort)))
	return srv, nil
}

func mountSwagger(r chi.Router) {
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		if _, errWrite := w.Write(contracts.Bot); errWrite != nil {
			slog.Error("run: swagger yaml write error", slog.String("error", errWrite.Error()))
		}
	})
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, errWrite := w.Write([]byte(botswagger.SwaggerHTML)); errWrite != nil {
			slog.Error("run: swagger html write error", slog.String("error", errWrite.Error()))
		}
	})
}
