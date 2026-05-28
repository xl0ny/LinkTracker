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
	botopenapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	botredis "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/redis"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/scrapperclient"
	botswagger "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/swagger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	transporthttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http"
	botapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/kafka"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		slog.Error("run: bot initialization error", slog.String("error", err.Error()))
		return fmt.Errorf("telegram bot: %w", err)
	}

	tracker, err := scrapperclient.NewLinkTracker(cfg.ScrapperURL)
	if err != nil {
		return fmt.Errorf("app run: Link tracker creation error - %w", err)
	}

	kafkaConsumer, closeKafka, err := newKafkaConsumer(ctx, cfg.Kafka, cfg.Redis, bot)
	if err != nil {
		return err
	}
	defer closeKafka()
	if kafkaConsumer != nil {
		go func() {
			if rerr := kafkaConsumer.Run(ctx); rerr != nil {
				slog.Error("kafka consumer running error", slog.String("error", rerr.Error()), slog.String("event", "kafka"))
			}
		}()
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

	if httpErr := runBotHTTPServer(ctx, cfg.BotPort, newBotHTTPRouter(bot)); httpErr != nil {
		return httpErr
	}
	slog.Info("run: shutting down")
	return nil
}

func runBotHTTPServer(ctx context.Context, port string, handler http.Handler) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		slog.Error("run: http server bind error", slog.String("error", err.Error()), slog.String("port", port))
		return fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: handler}
	go func() {
		if errSrv := srv.Serve(ln); errSrv != nil && errSrv != http.ErrServerClosed {
			slog.Error("run: http server serve error", slog.String("error", errSrv.Error()))
		}
	}()
	swaggerUI := fmt.Sprintf("http://127.0.0.1:%s/swagger", port)
	slog.Info("run: http server started",
		slog.String("port", port),
		slog.String("swagger_ui", swaggerUI))

	<-ctx.Done()

	if shutErr := srv.Shutdown(context.Background()); shutErr != nil {
		slog.Error("run: server graceful shutdown error", slog.String("error", shutErr.Error()))
	}
	return nil
}

func newKafkaConsumer(ctx context.Context, kafkaCfg config.KafkaSettings, redisCfg config.RedisSettings, sender kafka.MessageSender) (*kafka.Consumer, func(), error) {
	cleanup := func() {}
	if !kafkaCfg.Cluster.Enabled {
		return nil, cleanup, nil
	}

	var idem kafka.IdempotencyStore
	if redisCfg.Enabled {
		redisStore := botredis.New(redisCfg)
		if perr := redisStore.Ping(ctx); perr != nil {
			return nil, nil, fmt.Errorf("bot run: redis ping: %w", perr)
		}
		prev := cleanup
		cleanup = func() {
			prev()
			if cerr := redisStore.Close(); cerr != nil {
				slog.Error("redis close error", slog.String("error", cerr.Error()), slog.String("event", "redis"))
			}
		}
		idem = redisStore
	}

	consumer, kerr := kafka.NewConsumer(kafkaCfg.Cluster, kafkaCfg.Consumer, sender, idem)
	if kerr != nil {
		cleanup()
		return nil, nil, fmt.Errorf("bot run: kafka consumer: %w", kerr)
	}
	prev := cleanup
	cleanup = func() {
		prev()
		if cerr := consumer.Close(); cerr != nil {
			slog.Error("kafka consumer close error", slog.String("error", cerr.Error()), slog.String("event", "kafka"))
		}
	}

	return consumer, cleanup, nil
}

func newBotHTTPRouter(sender transporthttp.MessageSender) http.Handler {
	r := chi.NewRouter()
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		if _, werr := w.Write(botopenapi.ContractYAML); werr != nil {
			slog.Error("swagger yaml write error", slog.String("error", werr.Error()), slog.String("event", "swagger"))
		}
	})
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, werr := w.Write([]byte(botswagger.SwaggerHTML)); werr != nil {
			slog.Error("swagger html write error", slog.String("error", werr.Error()), slog.String("event", "swagger"))
		}
	})
	botapi.HandlerFromMux(transporthttp.NewHandler(sender), r)
	return r
}
