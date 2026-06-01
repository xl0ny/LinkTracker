package metricssender

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

type Sender interface {
	SendMessage(chatid int64, message string) error
}

type notifying struct {
	inner Sender
	m     *metrics.Bot
}

func Wrap(inner Sender, m *metrics.Bot) Sender {
	if m == nil {
		return inner
	}
	return &notifying{inner: inner, m: m}
}

func (n *notifying) SendMessage(chatid int64, message string) error {
	err := n.inner.SendMessage(chatid, message)
	if err == nil {
		n.m.SentNotifications.Inc()
	}
	return err
}
