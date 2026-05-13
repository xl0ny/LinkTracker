# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/scrapper ./cmd/scrapper \
	&& CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/scrapper /out/bot /app/
COPY cmd/scrapper/config.yaml /app/cmd/scrapper/config.yaml
COPY cmd/bot/config.yaml /app/cmd/bot/config.yaml
