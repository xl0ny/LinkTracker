package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/httputil/helper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
)

type UseCase interface {
	ChatRegistration(ctx context.Context, id int64) error
	ChatDelition(ctx context.Context, id int64) error
	LinkAddment(ctx context.Context, chatID int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
}

type Handler struct {
	uc UseCase
}

func NewHandler(uc UseCase) *Handler {
	return &Handler{
		uc: uc,
	}
}

// Method names PostTgChatId/DeleteTgChatId are required by generated OpenAPI server interface.
//
//nolint:revive,staticcheck // method names must match generated ServerInterface
func (h *Handler) PostTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.uc.ChatRegistration(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrChatAlreadyExists) {
			helper.WriteError(
				w,
				http.StatusConflict,
				"failed to registrate chat",
				"CHAT_ALREADY_EXISTS",
				"Чат уже зарегистрирован",
				"ErrChatAlreadyExists",
				err.Error(),
			)
			return
		}
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to registrate chat",
			"INTERNAL_ERROR",
			"Внутренняя ошибка",
			"ErrInternal",
			err.Error(),
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

//nolint:revive,staticcheck // method name must match generated ServerInterface
func (h *Handler) DeleteTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.uc.ChatDelition(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			helper.WriteError(
				w,
				http.StatusNotFound,
				"failed to delete chat",
				"CHAT_NOT_FOUND",
				"Чат не найден",
				"ErrChatNotFound",
				err.Error(),
			)
			return
		}
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to delete chat",
			"INTERNAL_ERROR",
			"Внутренняя ошибка",
			"ErrInternal",
			err.Error(),
		)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) PostLinks(w http.ResponseWriter, r *http.Request, params api.PostLinksParams) {
	var body api.AddLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"invalid request body",
			"BAD_REQUEST",
			"Некорректное тело запроса",
			"ErrBadRequest",
			err.Error(),
		)
		return
	}

	if body.Link == nil || *body.Link == "" {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"invalid request body",
			"BAD_REQUEST",
			"Ссылка обязательна",
			"ErrBadRequest",
			"link is required",
		)
		return
	}
	if err := h.uc.LinkAddment(r.Context(), params.TgChatId, *body.Link, body.Tags, body.Filters); err != nil {
		if errors.Is(err, domain.ErrLinkAlreadyExists) {
			helper.WriteError(
				w,
				http.StatusConflict,
				"failed to add link",
				"LINK_ALREADY_EXISTS",
				"Ссылка уже отслеживается",
				"ErrLinkAlreadyExists",
				err.Error(),
			)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			helper.WriteError(
				w,
				http.StatusNotFound,
				"failed to add link",
				"CHAT_NOT_FOUND",
				"Чат не найден",
				"ErrChatNotFound",
				err.Error(),
			)
			return
		}
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to add link",
			"INTERNAL_ERROR",
			"Внутренняя ошибка",
			"ErrInternal",
			err.Error(),
		)
		return
	}
	url, tags, filters := *body.Link, []string{}, []string{}
	if body.Tags != nil {
		tags = *body.Tags
	}
	if body.Filters != nil {
		filters = *body.Filters
	}
	helper.WriteJSON(w, http.StatusOK, api.LinkResponse{
		Url:     &url,
		Tags:    &tags,
		Filters: &filters,
	})
}

func (h *Handler) GetLinks(w http.ResponseWriter, r *http.Request, params api.GetLinksParams) {
	links, err := h.uc.GetLinks(r.Context(), params.TgChatId)
	if err != nil && errors.Is(err, domain.ErrChatNotFound) {
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to get links",
			"INTERNAL_ERROR",
			"Внутренняя ошибка",
			"ErrInternal",
			err.Error(),
		)
		return
	}
	apiLinks := make([]api.LinkResponse, len(links))
	for i, l := range links {
		url, tags, filters := l.URL, l.Tags, l.Filters
		apiLinks[i] = api.LinkResponse{
			Url:     &url,
			Tags:    &tags,
			Filters: &filters,
		}
	}
	size := int32(len(apiLinks))
	helper.WriteJSON(w, http.StatusOK, api.ListLinksResponse{
		Links: &apiLinks,
		Size:  &size,
	})
}

func (h *Handler) DeleteLinks(w http.ResponseWriter, r *http.Request, params api.DeleteLinksParams) {
	var body api.RemoveLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"invalid request body",
			"BAD_REQUEST",
			"Некорректное тело запроса",
			"ErrBadRequest",
			err.Error(),
		)
		return
	}
	if body.Link == nil || *body.Link == "" {
		helper.WriteError(
			w,
			http.StatusBadRequest,
			"link is required",
			"BAD_REQUEST",
			"Ссылка обязательна",
			"ErrBadRequest",
			"link is required",
		)
		return
	}
	removed, err := h.uc.DeleteLink(r.Context(), params.TgChatId, *body.Link)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) || errors.Is(err, domain.ErrLinkNotFound) {
			helper.WriteError(
				w,
				http.StatusNotFound,
				"failed to remove link",
				"NOT_FOUND",
				"Чат не найден или ссылка не найдена",
				"ErrNotFound",
				err.Error(),
			)
			return
		}
		helper.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to remove link",
			"INTERNAL_ERROR",
			"Внутренняя ошибка",
			"ErrInternal",
			err.Error(),
		)
		return
	}
	url, tags, filters := removed.URL, removed.Tags, removed.Filters
	helper.WriteJSON(w, http.StatusOK, api.LinkResponse{
		Url:     &url,
		Tags:    &tags,
		Filters: &filters,
	})
}
