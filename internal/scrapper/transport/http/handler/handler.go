package handler

import (
	"context"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/httputil/helper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
)

type UseCase interface {
	chatRegistration(ctx context.Context, id int64) error
	chatDelition(ctx context.Context, id int64) error
	linkAddment(ctx context.Context, chatId int64) error
}

type Handler struct {
	uc UseCase
}

func NewHandler(uc UseCase) *Handler {
	return &Handler{
		uc: uc,
	}
}

func (h *Handler) PostTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.uc.chatRegistration(r.Context(), id); err.Error() == "chat already exists" {
		slog.Error("failed to registrate chat", slog.String("error", err.Error()))
		code := "CHAT_ALREADY_EXISTS"
		desc := "Чат уже зарегистрирован"
		excp := "ErrChatAlreadyExists"
		helper.WriteJSON(w, 409, api.ApiErrorResponse{
			Code:             &code,
			Description:      &desc,
			ExceptionMessage: helper.Ptr(err.Error()),
			ExceptionName:    &excp,
		})
	}
	w.WriteHeader(200)

}

func (h *Handler) DeleteTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.uc.chatDelition(r.Context(), id); err.Error() == "chat not found" {
		slog.Error("failed to delete chat", slog.String("error", err.Error()))
		code := "CHAT_NOT_FOUND"
		desc := "Чат не найден"
		excp := "ErrChatNotFound"
		helper.WriteJSON(w, 409, api.ApiErrorResponse{
			Code:             &code,
			Description:      &desc,
			ExceptionMessage: helper.Ptr(err.Error()),
			ExceptionName:    &excp,
		})
		w.WriteHeader(200)
	}
}

func (h *Handler) PostLinks(w http.ResponseWriter, r *http.Request, params api.PostLinksParams) {
	if err := h.uc.linkAddment(r.Context(), params.TgChatId); err.Error() == "" {

	}
}
