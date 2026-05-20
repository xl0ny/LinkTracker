package transporthttp

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/http/api"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/httputil/helper"
)

type MessageSender interface {
	SendMessage(chatID int64, message string) error
}

type Handler struct {
	sender MessageSender
}

func NewHandler(sender MessageSender) *Handler {
	return &Handler{sender: sender}
}

func (h *Handler) PostUpdates(w http.ResponseWriter, r *http.Request) {
	slog.Info("http: POST /updates received")
	var body api.LinkUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"http: invalid updates body",
			"BAD_REQUEST",
			"Некорректное тело запроса",
			"ErrInvalidJSON",
			err.Error(),
		)
		return
	}
	if body.TgChatIds == nil || len(*body.TgChatIds) == 0 {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"http: missing tgChatIds",
			"BAD_REQUEST",
			"Параметр tgChatIds обязателен и не может быть пустым",
			"ErrValidation",
			"tgChatIds is required and must be non-empty",
		)
		return
	}
	msg := "Обновление обнаружено"
	if body.Url != nil && *body.Url != "" {
		msg = "Обновление: " + *body.Url
	}
	if body.Description != nil && *body.Description != "" {
		msg = *body.Description
	}
	var sendErr bool
	for _, chatID := range *body.TgChatIds {
		if err := h.sender.SendMessage(chatID, msg); err != nil {
			slog.Warn("http: send update error", slog.Int64("chat_id", chatID), slog.String("error", err.Error()))
			sendErr = true
		} else {
			slog.Info("http: update sent", slog.Int64("chat_id", chatID))
		}
	}
	if sendErr {
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"http: send update failed",
			"INTERNAL_ERROR",
			"Не удалось отправить сообщение в один или несколько чатов",
			"ErrSendMessage",
			"telegram send failed for at least one chat",
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}
