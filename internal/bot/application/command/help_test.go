package command

import (
	"strings"
	"testing"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestHelp_Name(t *testing.T) {
	var cmd Help
	if got := cmd.Name(); got != "help" {
		t.Errorf("Name() = %q, want %q", got, "help")
	}
}

func TestHelp_Description(t *testing.T) {
	var cmd Help
	if got := cmd.Description(); got == "" {
		t.Error("Description() must be non-empty")
	}
}

func TestHelp_Handle_returnsCommandList(t *testing.T) {
	var cmd Help
	action := domain.Action{Command: "help"}
	text, send := cmd.Handle(action)
	if !send {
		t.Error("Handle() must return send=true")
	}
	if !strings.Contains(text, "/start") || !strings.Contains(text, "/help") {
		t.Errorf("Handle() must contain /start and /help, got %q", text)
	}
}
