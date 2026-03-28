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
	config, err := db.GetConfig()

	if err != nil {
		log.Println("migrator: config loading failed:", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	srcURL := "file://" + filepath.ToSlash(migrationsDir)

	m, err := migrate.New(srcURL, dsn)
	if err != nil {
		log.Println("migrator init failed:", err)
		os.Exit(1)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			log.Println("migrator close:", srcErr, dbErr)
		}
	}()

	err = runMigration(m, config.Migrations.Direction, config.Migrations.Steps)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Println("migration failed:", err)
		os.Exit(1)
	}
	log.Println("migration done")

}

func runMigration(m *migrate.Migrate, dir string, steps int) error {
	switch dir {
	case "up":
		if steps > 0 {
			return m.Steps(steps)
		}
		return m.Up()
	case "down":
		if steps > 0 {
			return m.Steps(-steps)
		}
		return m.Down()
	default:
		return fmt.Errorf("unknown direction: %s", dir)
	}
}
