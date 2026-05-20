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

	commondb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/db"
)

func main() {
	os.Exit(run())
}

func run() int {
	config, err := loadConfig()
	if err != nil {
		log.Println("migrator: config loading failed:", err)
		return 1
	}

	dsn := commondb.BuildPostgresDSN(
		config.DB.PostgresUser,
		config.PostgresPassword,
		config.DB.PostgresHost,
		config.DB.PostgresPort,
		config.DB.PostgresDB,
		config.DB.PostgresSSLMode,
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

	if migErr := migrateToTarget(m, resolveTargetVersion(config.Migrations.TargetVersion)); migErr != nil && !errors.Is(migErr, migrate.ErrNoChange) {
		log.Println("migration failed:", migErr)
		return 1
	}
	log.Println("migration done")
	return 0
}

func resolveTargetVersion(v *int) int {
	if v == nil {
		return -1
	}
	return *v
}

func migrateToTarget(m *migrate.Migrate, target int) error {
	switch {
	case target < -1:
		return errors.New("migrations: target_version must be >= -1 (-1=all up, 0=all down, N>=1=exact version)")
	case target == -1:
		if err := m.Up(); err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		return nil
	case target == 0:
		if err := m.Down(); err != nil {
			return fmt.Errorf("migrate down: %w", err)
		}
		return nil
	default:
		if err := m.Migrate(uint(target)); err != nil {
			return fmt.Errorf("migrate to version %d: %w", target, err)
		}
		return nil
	}
}
