package contracts

import _ "embed"

// Bot — OpenAPI-контракт сервиса Bot (bot.yaml).
//
//go:embed bot.yaml
var Bot []byte

// Scrapper — OpenAPI-контракт сервиса Scrapper (scrapper.yaml).
//
//go:embed scrapper.yaml
var Scrapper []byte
