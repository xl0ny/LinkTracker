package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"

	scrapcfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

type envPostgres struct {
	PostgresUser     string `envconfig:"POSTGRES_USER" required:"true"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" required:"true"`
	PostgresDB       string `envconfig:"POSTGRES_DB" required:"true"`
	PostgresHost     string `envconfig:"POSTGRES_HOST" required:"true"`
	PostgresPort     string `envconfig:"POSTGRES_PORT" required:"true"`
	PostgresSSLMode  string `envconfig:"POSTGRES_SSL_MODE" required:"true"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("gormgen: .env not loaded (optional):", err)
	}

	var cfg envPostgres
	if err := envconfig.Process("", &cfg); err != nil {
		log.Println("gormgen: env:", err)
		os.Exit(1)
	}

	dsn := scrapcfg.BuildPostgresDSN(
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
		cfg.PostgresSSLMode,
	)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Println("gormgen: gorm open:", err)
		os.Exit(1)
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Println("gormgen: runtime.Caller failed")
		os.Exit(1)
	}

	ormRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), ".."))
	if err != nil {
		log.Println("gormgen: orm root:", err)
		os.Exit(1)
	}

	queryDir := filepath.Join(ormRoot, "query")
	modelDir := filepath.Join(ormRoot, "model")

	g := gen.NewGenerator(gen.Config{
		OutPath:      queryDir,
		ModelPkgPath: modelDir,
		Mode:         gen.WithoutContext,
	})

	g.UseDB(gormDB)
	g.ApplyBasic(
		g.GenerateModel("chats"),
		g.GenerateModel("subscriptions"),
		g.GenerateModel("links"),
		g.GenerateModel("tag"),
		g.GenerateModel("filter"),
		g.GenerateModel("link_tag"),
		g.GenerateModel("link_filter"),
	)
	g.Execute()

	log.Println("gormgen: models ->", modelDir)
	log.Println("gormgen: query  ->", queryDir, "(orm/repo.go imports query for single-table CRUD; regenerate after schema changes)")
}
