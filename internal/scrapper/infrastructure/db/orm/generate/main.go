package main

import (
	"log"
	"path/filepath"
	"runtime"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Println("gormgen: config:", err)
		return
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.PostgresDSN()), &gorm.Config{})
	if err != nil {
		log.Println("gormgen: gorm open:", err)
		return
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Println("gormgen: runtime.Caller failed")
		return
	}

	ormRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), ".."))
	if err != nil {
		log.Println("gormgen: orm root:", err)
		return
	}

	queryDir := filepath.Join(ormRoot, "query")
	modelDir := filepath.Join(ormRoot, "model")

	g := gen.NewGenerator(gen.Config{
		OutPath:      queryDir,
		ModelPkgPath: modelDir,
		Mode:         gen.WithoutContext,
	})

	g.UseDB(gormDB)
	g.ApplyBasic(g.GenerateAllTable()...)
	g.Execute()

	log.Println("gormgen: models ->", modelDir)
	log.Println("gormgen: query  ->", queryDir, "(orm/repo.go imports query for single-table CRUD; regenerate after schema changes)")
}
