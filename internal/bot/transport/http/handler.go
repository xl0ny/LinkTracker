package http

import "net/http"

type UseCase interface {
	// func()
}

type Handler struct {
	uc UseCase
}

func (h *Handler) PostUpdates(w http.ResponseWriter, r *http.Request) {

}
