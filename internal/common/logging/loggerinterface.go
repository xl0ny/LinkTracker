package logging

import (
	"log/slog"
)

type LoggerI interface {
	slog.Handler
}
