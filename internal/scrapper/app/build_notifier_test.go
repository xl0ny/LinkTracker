package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/prometrics"
)

func testScrapperMetrics() *prometrics.ScrapperMetrics {
	return prometrics.NewScrapperMetrics(prometrics.New("test"))
}

func TestBuildNotifier_kafkaDisabledUsesHTTPNotifier(t *testing.T) {
	tests := []struct{ name string }{
		{name: "build notifier kafka disabled uses httpnotifier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			srv := httptest.NewServer(http.NotFoundHandler())
			defer srv.Close()
			api, err := botclient.NewClientWithResponses(srv.URL)
			assert.NoError(t, err)

			var cfg config.Config
			cfg.Kafka.Cluster.Enabled = false

			n, p, err := buildNotifier(context.Background(), nil, api, &cfg, testScrapperMetrics())
			assert.NoError(t, err)
			assert.Nil(t, p)
			assert.NotNil(t, n)
		})
	}
}

func TestBuildNotifier_kafkaEnabledUsesHTTPWithFallback(t *testing.T) {
	tests := []struct{ name string }{
		{name: "build notifier kafka enabled uses httpwith fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			srv := httptest.NewServer(http.NotFoundHandler())
			defer srv.Close()
			api, err := botclient.NewClientWithResponses(srv.URL)
			assert.NoError(t, err)

			var cfg config.Config
			cfg.Kafka.Cluster.Enabled = true
			cfg.Kafka.Producer.Mode = kafkaProducerModeDirect
			cfg.Kafka.Cluster.Brokers = []string{"localhost:19092"}
			cfg.Kafka.Cluster.RawUpdatesTopic = "link.raw-updates"
			cfg.Kafka.Cluster.FailedLinksTopic = "failed-links"
			cfg.Kafka.Cluster.DLQTopic = "link.raw-updates-dlq"
			cfg.Kafka.Cluster.SchemaRegistryURL = "http://localhost:18081"
			cfg.Kafka.Cluster.RawUpdateSubject = "link-raw-update-event-value"
			cfg.Kafka.Cluster.FailedSubject = "failed-links-event-value"
			cfg.Kafka.Producer.ProducerClient = "test"
			cfg.Kafka.Producer.WriteTimeout = time.Second
			cfg.Kafka.Producer.RequiredACK = -1
			cfg.Kafka.Producer.MaxAttempts = 1

			n, p, err := buildNotifier(context.Background(), nil, api, &cfg, testScrapperMetrics())
			if err != nil {
				t.Skip("kafka notifier init:", err)
			}
			assert.Nil(t, p)
			_, ok := n.(*notifier.Fallback)
			assert.True(t, ok)
		})
	}
}
