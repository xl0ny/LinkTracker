package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/prometrics"
)

func testScrapperMetrics() *prometrics.Scrapper {
	return prometrics.NewScrapper(prometrics.New("test"))
}

func TestBuildNotifier_kafkaDisabledUsesHTTPNotifier(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	api, err := botclient.NewClientWithResponses(srv.URL)
	require.NoError(t, err)

	var cfg config.Config
	cfg.Kafka.Kafka.Enabled = false

	n, p, err := buildNotifier(context.Background(), nil, api, &cfg, testScrapperMetrics())
	require.NoError(t, err)
	require.Nil(t, p)
	require.NotNil(t, n)
}

func TestBuildNotifier_kafkaEnabledUsesHTTPWithFallback(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	api, err := botclient.NewClientWithResponses(srv.URL)
	require.NoError(t, err)

	var cfg config.Config
	cfg.Kafka.Kafka.Enabled = true
	cfg.Kafka.Producer.Mode = kafkaProducerModeDirect
	cfg.Kafka.Kafka.Brokers = []string{"localhost:19092"}
	cfg.Kafka.Kafka.RawUpdatesTopic = "link.raw-updates"
	cfg.Kafka.Kafka.FailedLinksTopic = "failed-links"
	cfg.Kafka.Kafka.DLQTopic = "link.raw-updates-dlq"
	cfg.Kafka.Kafka.SchemaRegistryURL = "http://localhost:18081"
	cfg.Kafka.Kafka.RawUpdateSubject = "link-raw-update-event-value"
	cfg.Kafka.Kafka.FailedSubject = "failed-links-event-value"
	cfg.Kafka.Producer.ProducerClient = "test"
	cfg.Kafka.Producer.WriteTimeout = time.Second
	cfg.Kafka.Producer.RequiredACK = -1
	cfg.Kafka.Producer.MaxAttempts = 1

	n, p, err := buildNotifier(context.Background(), nil, api, &cfg, testScrapperMetrics())
	if err != nil {
		t.Skip("kafka notifier init:", err)
	}
	require.Nil(t, p)
	_, ok := n.(*notifier.Fallback)
	require.True(t, ok)
}
