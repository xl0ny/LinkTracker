package handler

import "net/http"

type UseCase interface {
	chatRegistration(id int)
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
	h.uc.chatRegistration(id)
}
