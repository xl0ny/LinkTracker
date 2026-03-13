package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/scrapperclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	bothttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http"
	botapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http/api"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot := telegram.NewBot(cfg.TelegramToken)

	tracker, err := scrapperclient.NewLinkTracker(cfg.ScrapperURL)
	if err != nil {
		slog.Warn("scrapper client disabled", slog.String("error", err.Error()), slog.String("event", "scrapper_client"))
		tracker = application.NewNoopTracker()
	}
	stateStore := application.NewTrackStateStore()
	commands, trackCmd := command.All(tracker, stateStore)
	bot.SetMenuCommands(commands)

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands, trackCmd, stateStore)

	for range workerCount {
		go application.Worker(actions, dispatcher, bot)
	}

	r := chi.NewRouter()
	updatesHandler := bothttp.NewHandler(bot)
	botapi.HandlerFromMux(updatesHandler, r)
	ln, err := net.Listen("tcp", ":"+cfg.BotPort)
	if err != nil {
		slog.Error("http server bind failed", slog.String("error", err.Error()), slog.String("port", cfg.BotPort), slog.String("event", "http_server"))
		os.Exit(1)
	}
	srv := &http.Server{Handler: r}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", slog.String("error", err.Error()), slog.String("event", "http_server"))
		}
	}()
	slog.Info("http server started", slog.String("port", cfg.BotPort), slog.String("event", "http_started"))

	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
	slog.Info("shutting down", slog.String("event", "shutdown"))
	os.Exit(0)
}
