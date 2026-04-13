package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/db"
)

func main() {
	os.Exit(run())
}

func run() int {
	config, err := db.GetConfig()
	if err != nil {
		log.Println("migrator: config loading failed:", err)
		return 1
	}

	dsn := db.BuildPostgresDSN(
		config.PostgresUser,
		config.PostgresPassword,
		config.PostgresHost,
		config.PostgresPort,
		config.PostgresDB,
		config.PostgresSSLMode,
	)

	migrationsDir, err := filepath.Abs(config.Migrations.Dir)
	if err != nil {
		log.Println("migrator: migrations path:", err)
		return 1
	}
	srcURL := "file://" + filepath.ToSlash(migrationsDir)

	m, err := migrate.New(srcURL, dsn)
	if err != nil {
		log.Println("migrator init failed:", err)
		return 1
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			log.Println("migrator close:", srcErr, dbErr)
		}
	}()

	if migErr := runMigration(m, config.Migrations.Direction, config.Migrations.Steps); migErr != nil && !errors.Is(migErr, migrate.ErrNoChange) {
		log.Println("migration failed:", migErr)
		return 1
	}
	log.Println("migration done")
	return 0
}

func runMigration(m *migrate.Migrate, dir string, steps int) error {
	switch dir {
	case "up":
		if steps > 0 {
			if err := m.Steps(steps); err != nil {
				return fmt.Errorf("migrate steps up: %w", err)
			}
			return nil
		}
		if err := m.Up(); err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		return nil
	case "down":
		if steps > 0 {
			if err := m.Steps(-steps); err != nil {
				return fmt.Errorf("migrate steps down: %w", err)
			}
			return nil
		}
		if err := m.Down(); err != nil {
			return fmt.Errorf("migrate down: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown direction: %s", dir)
	}
}
