package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/botclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

func TestBuildNotifier_kafkaDisabledUsesHTTPNotifier(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	api, err := botclient.NewClientWithResponses(srv.URL)
	require.NoError(t, err)

	var cfg config.Config
	cfg.Kafka.Kafka.Enabled = false

	n, p, err := buildNotifier(context.Background(), nil, api, &cfg)
	require.NoError(t, err)
	require.Nil(t, p)
	require.NotNil(t, n)
}
