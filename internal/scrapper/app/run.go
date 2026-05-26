package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/go-chi/chi/v5"
	scrapperapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/pgrepo"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/outbox"
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
		if _, werr := w.Write(scrapperapi.ContractYAML); werr != nil {
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
	gh := github.NewClient(nil, os.Getenv("GITHUB_TOKEN"))
	so := stackoverflow.NewClient(nil)
	lc := checker.New(gh, so)

	notifier, publisher, npErr := buildNotifier(ctx, repo, botAPI, cfg)
	if npErr != nil {
		return npErr
	}

	sch := application.NewScheduler(
		repo,
		lc,
		notifier,
		cfg.Batch.Size,
		cfg.Scheduler.Workers,
		cfg.Scheduler.Interval,
	)
	if publisher != nil {
		if tr, ok := repo.(application.TxRunner); ok {
			sch.SetTxRunner(tr)
		}
		go publisher.Run(ctx)
		defer func() {
			if cerr := publisher.Close(); cerr != nil {
				slog.Error("scrapper run: outbox publisher close", slog.String("error", cerr.Error()))
			}
		}()
	}
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
	if shutErr := srv.Shutdown(context.Background()); shutErr != nil {
		return fmt.Errorf("shutdown: %w", shutErr)
	}
	return nil
}

func buildNotifier(ctx context.Context, repo Repository, botAPI *botclient.ClientWithResponses, cfg *config.Config) (application.BotNotifier, *outbox.Publisher, error) {
	if !cfg.Kafka.Cluster.Enabled {
		return botclient.NewNotifier(botAPI), nil, nil
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Kafka.Producer.Mode))
	if mode == "" {
		mode = "direct"
	}
	switch mode {
	case "direct":
		n, err := kafka.NewNotifier(ctx, cfg.Kafka.Cluster, cfg.Kafka.Producer.KafkaProducer)
		if err != nil {
			return nil, nil, fmt.Errorf("scrapper: kafka notifier: %w", err)
		}
		return n, nil, nil
	case "outbox":
		writeRepo, okWrite := repo.(outbox.NotifierRepository)
		pollRepo, okPoll := repo.(outbox.PublisherRepository)
		if !okWrite || !okPoll {
			return nil, nil, errors.New("scrapper: outbox mode requires a repository with outbox support (use access_type=SQL)")
		}
		notifier, err := outbox.NewNotifier(ctx, writeRepo, cfg.Kafka.Cluster)
		if err != nil {
			return nil, nil, fmt.Errorf("scrapper: outbox notifier: %w", err)
		}
		publisher := outbox.NewPublisher(pollRepo, cfg.Kafka.Cluster, cfg.Kafka.Producer.KafkaProducer, cfg.Kafka.Producer.Outbox)
		return notifier, publisher, nil
	default:
		return nil, nil, fmt.Errorf("scrapper: unknown kafka producer mode %q (expected direct|outbox)", cfg.Kafka.Producer.Mode)
	}
}

func newChatRepository(ctx context.Context, cfg *config.Config) (Repository, error) {
	mode := strings.ToUpper(strings.TrimSpace(cfg.AccessType))
	if mode == "" {
		mode = "SQL"
	}
	dsn := cfg.PostgresDSN()
	switch mode {
	case "SQL":
		repo, sqlErr := pgrepo.NewRepository(ctx, dsn)
		if sqlErr != nil {
			return nil, fmt.Errorf("scrapper: pgrepo repository: %w", sqlErr)
		}
		return repo, nil
	case "ORM":
		repo, ormErr := orm.NewRepository(ctx, dsn)
		if ormErr != nil {
			return nil, fmt.Errorf("scrapper: orm repository: %w", ormErr)
		}
		return repo, nil
	default:
		return nil, fmt.Errorf("scrapper: unknown access_type %q (use SQL or ORM)", cfg.AccessType)
	}
}
