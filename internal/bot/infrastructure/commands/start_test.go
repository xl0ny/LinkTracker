package commands_test

import (
	"testing"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/commands"
)

func TestStart_Name(t *testing.T) {
	var cmd commands.Start
	if got := cmd.Name(); got != "start" {
		t.Errorf("Name() = %q, want %q", got, "start")
	}
}

func TestStart_Description(t *testing.T) {
	var cmd commands.Start
	if got := cmd.Description(); got == "" {
		t.Error("Description() must be non-empty")
	}
}

func TestStart_Handle_returnsWelcomeMessage(t *testing.T) {
	var cmd commands.Start
	action := domain.Action{Command: "start"}
	text, send := cmd.Handle(action, nil)
	if !send {
		t.Error("Handle() must return send=true")
	}
	want := "Добро пожаловать! Используйте /help для списка команд."
	if text != want {
		t.Errorf("Handle() text = %q, want %q", text, want)
	}
}
