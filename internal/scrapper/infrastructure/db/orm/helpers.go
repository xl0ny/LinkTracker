package orm

import (
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func closeSQLRows(rows *sql.Rows, op string) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		slog.Warn("orm repo: rows close", slog.String("op", op), slog.String("error", err.Error()))
	}
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
