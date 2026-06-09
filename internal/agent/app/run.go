package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/config"
	agentkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/summarizer"
)

func Run(ctx context.Context, cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if !cfg.Kafka.Cluster.Enabled {
		return errors.New("agent run: kafka must be enabled")
	}

	filter := application.NewFilter(application.FilterConfig{
		StopWords:       cfg.AIAgent.Filtering.StopWords,
		ExcludedAuthors: cfg.AIAgent.Filtering.ExcludedAuthors,
		MinLength:       cfg.AIAgent.Filtering.MinLength,
	})

	sum, err := buildSummarizer(cfg)
	if err != nil {
		return err
	}

	prioritizer := application.NewPrioritizer(application.PrioritizerConfig{
		HighKeywords: cfg.AIAgent.Prioritization.HighKeywords,
		LowKeywords:  cfg.AIAgent.Prioritization.LowKeywords,
	})
	processor := application.NewProcessor(filter, sum, cfg.AIAgent.Summarization.Threshold, prioritizer)

	consumer, closeKafka, err := buildKafkaPipeline(ctx, cfg, processor)
	if err != nil {
		return err
	}
	defer closeKafka()

	consumerErr := make(chan error, 1)
	go func() {
		consumerErr <- consumer.Run(ctx)
	}()

	httpDone := make(chan struct{})
	srv, err := startHealthServer(ctx, cfg.Port, httpDone)
	if err != nil {
		return err
	}

	slog.Info("agent run: started",
		slog.String("port", cfg.Port),
		slog.String("raw_topic", cfg.Kafka.Cluster.RawUpdatesTopic),
		slog.String("processed_topic", cfg.Kafka.Cluster.ProcessedUpdatesTopic),
		slog.String("summarizer", strings.ToLower(strings.TrimSpace(cfg.AIAgent.Summarization.Mode))),
		slog.Int("threshold", cfg.AIAgent.Summarization.Threshold),
	)

	select {
	case <-ctx.Done():
	case rerr := <-consumerErr:
		if rerr != nil {
			slog.Error("agent run: consumer terminated", slog.String("error", rerr.Error()))
		}
	}

	if shutErr := srv.Shutdown(context.Background()); shutErr != nil {
		slog.Error("agent run: http shutdown", slog.String("error", shutErr.Error()))
	}
	<-httpDone
	return nil
}

func buildKafkaPipeline(
	ctx context.Context,
	cfg *config.Config,
	processor agentkafka.Processor,
) (*agentkafka.Consumer, func(), error) {
	producer, err := agentkafka.NewProducer(ctx, cfg.Kafka.Cluster, cfg.Kafka.Producer.KafkaProducer)
	if err != nil {
		return nil, nil, fmt.Errorf("agent run: producer: %w", err)
	}

	windowMs := cfg.AIAgent.Grouping.WindowMs
	grouper := application.NewGrouper(producer, application.GrouperConfig{
		Window: time.Duration(windowMs) * time.Millisecond,
	})
	go grouper.Run(ctx)

	consumer, err := agentkafka.NewConsumer(cfg.Kafka.Cluster, cfg.Kafka.Consumer.KafkaConsumer, processor, grouper)
	if err != nil {
		_ = producer.Close()
		return nil, nil, fmt.Errorf("agent run: consumer: %w", err)
	}

	closeFn := func() {
		if cerr := consumer.Close(); cerr != nil {
			slog.Error("agent run: consumer close", slog.String("error", cerr.Error()))
		}
		if cerr := producer.Close(); cerr != nil {
			slog.Error("agent run: producer close", slog.String("error", cerr.Error()))
		}
	}
	return consumer, closeFn, nil
}

func buildSummarizer(cfg *config.Config) (application.Summarizer, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.AIAgent.Summarization.Mode))
	switch mode {
	case "", "stub":
		return summarizer.NewStub(cfg.AIAgent.Summarization.Threshold), nil
	case "huggingface":
		hfCfg := cfg.AIAgent.Summarization.HuggingFace
		if strings.TrimSpace(hfCfg.Token) == "" {
			slog.Warn("agent run: HUGGINGFACE_TOKEN is empty, falling back to stub summarizer")
			return summarizer.NewStub(cfg.AIAgent.Summarization.Threshold), nil
		}
		hf, err := summarizer.NewHuggingFace(summarizer.HuggingFaceConfig{
			APIURL:    hfCfg.APIURL,
			Token:     hfCfg.Token,
			MaxLength: hfCfg.MaxLength,
			MinLength: hfCfg.MinLength,
			Timeout:   hfCfg.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("agent run: huggingface summarizer: %w", err)
		}
		return hf, nil
	default:
		return nil, fmt.Errorf("agent run: unknown summarization mode %q", cfg.AIAgent.Summarization.Mode)
	}
}

func startHealthServer(ctx context.Context, port string, done chan<- struct{}) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("agent run: listen %s: %w", port, err)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		defer close(done)
		if sErr := srv.Serve(ln); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			slog.Error("agent run: http server error", slog.String("error", sErr.Error()))
		}
	}()
	return srv, nil
}
