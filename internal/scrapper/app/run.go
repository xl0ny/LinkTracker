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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/resilience"
	scrapperapi "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/pgrepo"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/outbox"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/swagger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/valkey"
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

	uc, closeCache, err := buildUseCase(ctx, repo, cfg)
	if err != nil {
		return err
	}
	defer closeCache()

	h := handler.NewHandler(uc)
	r := chi.NewRouter()
	r.Use(resilience.RateLimitMiddleware(cfg.Resilience.RateLimit))
	mountSwagger(r)
	api.HandlerFromMux(h, r)

	botHTTP := resilience.NewHTTPClient("bot", cfg.Resilience)
	botAPI, err := botclient.NewClientWithResponses(cfg.BotURL, botclient.WithHTTPClient(botHTTP))
	if err != nil {
		return fmt.Errorf("bot client init: %w", err)
	}
	ghHTTP := resilience.NewHTTPClient("github", cfg.Resilience)
	soHTTP := resilience.NewHTTPClient("stackoverflow", cfg.Resilience)
	gh := github.NewClient(ghHTTP, os.Getenv("GITHUB_TOKEN"))
	so := stackoverflow.NewClient(soHTTP)
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

	return runHTTPServer(ctx, r, cfg.Port)
}

func runHTTPServer(ctx context.Context, r chi.Router, port string) error {
	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		if errSrv := srv.ListenAndServe(); errSrv != nil && !errors.Is(errSrv, http.ErrServerClosed) {
			slog.Error("scrapper run: http server error", slog.String("error", errSrv.Error()))
		}
	}()
	slog.Info("scrapper run: http server started",
		slog.String("port", port),
		slog.String("swagger_ui", fmt.Sprintf("http://127.0.0.1:%s/swagger", port)))

	<-ctx.Done()
	if shutErr := srv.Shutdown(context.Background()); shutErr != nil {
		return fmt.Errorf("shutdown: %w", shutErr)
	}
	return nil
}

func mountSwagger(r chi.Router) {
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
}

func buildUseCase(ctx context.Context, repo Repository, cfg *config.Config) (handler.UseCase, func(), error) {
	base := application.NewChatUC(repo, repo)
	if !cfg.Valkey.Enabled {
		slog.Info("scrapper run: valkey cache disabled")
		return base, func() {}, nil
	}
	cache, err := valkey.New(ctx, valkey.Config{
		Addrs:       cfg.Valkey.Addrs,
		Password:    cfg.Valkey.Password,
		KeyPrefix:   cfg.Valkey.KeyPrefix,
		TTL:         cfg.Valkey.TTL,
		ClientCache: cfg.Valkey.ClientCache.Enabled,
		CSCTTL:      cfg.Valkey.ClientCache.TTL,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scrapper: valkey cache init: %w", err)
	}
	slog.Info("scrapper run: valkey cache enabled",
		slog.Bool("client_cache", cfg.Valkey.ClientCache.Enabled),
		slog.Int("addrs", len(cfg.Valkey.Addrs)))
	return application.NewCachedChatUC(base, cache), cache.Close, nil
}

func buildNotifier(ctx context.Context, repo Repository, botAPI *botclient.ClientWithResponses, cfg *config.Config) (application.BotNotifier, *outbox.Publisher, error) {
	primary := botclient.NewNotifier(botAPI)
	if !cfg.Kafka.Kafka.Enabled {
		return primary, nil, nil
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Kafka.Producer.Mode))
	if mode == "" {
		mode = "direct"
	}
	switch mode {
	case "direct":
		fallback, err := kafka.NewNotifier(ctx, cfg.Kafka.Kafka, cfg.Kafka.Producer.KafkaProducer)
		if err != nil {
			return nil, nil, fmt.Errorf("scrapper: kafka notifier: %w", err)
		}
		return notifier.NewFallback(primary, fallback), nil, nil
	case "outbox":
		writeRepo, okWrite := repo.(outbox.NotifierRepository)
		pollRepo, okPoll := repo.(outbox.PublisherRepository)
		if !okWrite || !okPoll {
			return nil, nil, errors.New("scrapper: outbox mode requires a repository with outbox support (use access_type=SQL)")
		}
		fallback, err := outbox.NewNotifier(ctx, writeRepo, cfg.Kafka.Kafka)
		if err != nil {
			return nil, nil, fmt.Errorf("scrapper: outbox notifier: %w", err)
		}
		publisher := outbox.NewPublisher(pollRepo, cfg.Kafka.Kafka, cfg.Kafka.Producer.KafkaProducer, cfg.Kafka.Producer.Outbox)
		return notifier.NewFallback(primary, fallback), publisher, nil
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
