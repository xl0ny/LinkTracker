package contracts

import _ "embed"

// Bot — OpenAPI-контракт сервиса Bot (contracts/bot.yaml).
//
//go:embed contracts/bot.yaml
var Bot []byte

// Scrapper — OpenAPI-контракт сервиса Scrapper (contracts/scrapper.yaml).
//
//go:embed contracts/scrapper.yaml
var Scrapper []byte
