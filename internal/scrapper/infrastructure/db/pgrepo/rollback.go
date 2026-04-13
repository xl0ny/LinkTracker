package pgrepo

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

func rollbackUnlessCommitted(ctx context.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Warn("pgrepo: transaction rollback", slog.String("error", err.Error()))
	}
}
