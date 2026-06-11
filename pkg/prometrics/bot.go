package prometrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ScopeScrapperSync  = "scrapper_sync_api"
	ScopeScrapperAsync = "scrapper_async_api"
)

type Bot struct {
	CommandRequests   *prometheus.CounterVec
	CommandDuration   *prometheus.HistogramVec
	SentNotifications prometheus.Counter
	TelegramRequests  *prometheus.CounterVec
	RED               *RED
}

func NewBot(reg *Registry) *Bot {
	labels := prometheus.Labels{"app": reg.App}
	b := &Bot{
		CommandRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "command_requests_total",
			Help:        "Processed bot commands",
			ConstLabels: labels,
		}, []string{"command"}),
		CommandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "command_duration_ms_total",
			Help:        "Command-related operation duration in milliseconds",
			ConstLabels: labels,
			Buckets:     durationMillisecondsBuckets,
		}, []string{"scope", "scope_type"}),
		SentNotifications: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "sent_notification_total",
			Help:        "Sent notifications",
			ConstLabels: labels,
		}),
		TelegramRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "telegram_requests_total",
			Help:        "Incoming Telegram user requests",
			ConstLabels: labels,
		}, []string{"request_type"}),
		RED: NewRED(reg.App),
	}
	reg.MustRegister(
		b.CommandRequests,
		b.CommandDuration,
		b.SentNotifications,
		b.TelegramRequests,
		b.RED.Requests,
		b.RED.Duration,
	)
	return b
}

func (b *Bot) IncCommand(command string) {
	b.CommandRequests.WithLabelValues(command).Inc()
}
