package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

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

func Run(ctx context.Context, cfg *config.Config) error {
	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("error on running bot stage: %w", err)
	}

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
	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			application.Worker(actions, dispatcher, bot)
		})
	}

	r := chi.NewRouter()
	updatesHandler := bothttp.NewHandler(bot)
	botapi.HandlerFromMux(updatesHandler, r)
	ln, err := net.Listen("tcp", ":"+cfg.BotPort)
	if err != nil {
		return fmt.Errorf("http server bind: %w", err)
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
	wg.Wait()
	return nil
}
