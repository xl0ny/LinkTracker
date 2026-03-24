package helper

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// APIErrorResponse — общая структура ошибки API (OpenAPI).
type APIErrorResponse struct {
	Code             *string   `json:"code,omitempty"`
	Description      *string   `json:"description,omitempty"`
	ExceptionMessage *string   `json:"exceptionMessage,omitempty"`
	ExceptionName    *string   `json:"exceptionName,omitempty"`
	Stacktrace       *[]string `json:"stacktrace,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("http helper: encode JSON response error", slog.String("error", err.Error()))
	}
}

func WriteError(w http.ResponseWriter, code int, slogname, codename, desc, excpn, err string) {
	slog.Error(slogname, slog.String("error", err))
	WriteJSON(w, code, APIErrorResponse{
		Code:             &codename,
		Description:      &desc,
		ExceptionMessage: &err,
		ExceptionName:    &excpn,
	})
}

func Ptr[T any](v T) *T { return &v }
