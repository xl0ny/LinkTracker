package main

import (
	"log"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot"
)

func main() {
	cfg, err := bot.Load()
	if err != nil {
		log.Fatal(err)
	}
	bot.Run(cfg)
}
