package app

import (
	"context"
	"fmt"
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

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		slog.Error("run: bot initialization error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	tracker, err := scrapperclient.NewLinkTracker(cfg.ScrapperURL)
	if err != nil {
		return fmt.Errorf("app run: Link tracker creation error - %w", err)
	}

	stateStore := application.NewTrackStateStore()
	trackCmd := command.NewTrack(tracker, stateStore)
	commands := command.Commands(tracker, trackCmd)
	bot.SetMenuCommands(commands)

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands, trackCmd, stateStore)

	for range workerCount {
		go application.Worker(actions, dispatcher, bot)
	}

	r := chi.NewRouter()
	updatesHandler := bothttp.NewHandler(bot)
	botapi.HandlerFromMux(updatesHandler, r)

	var lc net.ListenConfig
	ln, errListen := lc.Listen(ctx, "tcp", ":"+cfg.BotPort)
	if errListen != nil {
		slog.Error("run: http server bind error", slog.String("error", errListen.Error()), slog.String("port", cfg.BotPort))
		return fmt.Errorf("listen: %w", errListen)
	}

	srv := &http.Server{Handler: r}
	go func() {
		if errSrv := srv.Serve(ln); errSrv != nil && errSrv != http.ErrServerClosed {
			slog.Error("run: http server serve error", slog.String("error", errSrv.Error()))
		}
	}()
	slog.Info("run: http server started", slog.String("port", cfg.BotPort))

	<-ctx.Done()

	err = srv.Shutdown(context.Background())
	if err != nil {
		slog.Error("run: server graceful shutdown error", slog.String("error", err.Error()))
	}
	slog.Info("run: shutting down")
	return nil
}
