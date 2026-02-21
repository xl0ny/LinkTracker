package commands_test

import (
	"strings"
	"testing"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

func TestHelp_Name(t *testing.T) {
	var cmd commands.Help
	if got := cmd.Name(); got != "help" {
		t.Errorf("Name() = %q, want %q", got, "help")
	}
}

func TestHelp_Description(t *testing.T) {
	var cmd commands.Help
	if got := cmd.Description(); got == "" {
		t.Error("Description() must be non-empty")
	}
}

// Позитивный сценарий (blackbox): при получении /help бот отвечает описанием команд.
func TestHelp_Handle_returnsCommandList(t *testing.T) {
	var cmd commands.Help
	action := model.Action{Command: "help"}
	text, send := cmd.Handle(action, nil)
	if !send {
		t.Error("Handle() must return send=true")
	}
	if !strings.Contains(text, "/start") || !strings.Contains(text, "/help") {
		t.Errorf("Handle() must contain /start and /help, got %q", text)
	}
}
