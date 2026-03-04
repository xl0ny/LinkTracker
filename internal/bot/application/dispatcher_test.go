package application_test

import (
	"strings"
	"testing"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	commands "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	d := application.NewDispatcher(commands.All())
	action := domain.Action{Command: "start"}
	text, send := d.Dispatch(action)
	if !send {
		t.Fatal("Dispatch(start) must return send=true")
	}
	want := "Добро пожаловать! Используйте /help"
	if !strings.Contains(text, want) {
		t.Errorf("Dispatch(start) = %q, want substring %q", text, want)
	}
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	d := application.NewDispatcher(commands.All())
	action := domain.Action{Command: "help"}
	text, send := d.Dispatch(action)
	if !send {
		t.Fatal("Dispatch(help) must return send=true")
	}
	if !strings.Contains(text, "/start") || !strings.Contains(text, "/help") {
		t.Errorf("Dispatch(help) must contain /start and /help, got %q", text)
	}
}

func TestDispatcher_Dispatch_unknownCommand_returnsErrorMessage(t *testing.T) {
	d := application.NewDispatcher(commands.All())
	action := domain.Action{Command: "unknowncommand"}
	text, send := d.Dispatch(action)
	if !send {
		t.Fatal("Dispatch(unknown) must return send=true (error message)")
	}
	if !strings.Contains(text, "Неизвестная команда") || !strings.Contains(text, "/help") {
		t.Errorf("Dispatch(unknown) must mention unknown command and /help, got %q", text)
	}
}

func TestDispatcher_Dispatch_emptyCommand_noResponse(t *testing.T) {
	d := application.NewDispatcher(commands.All())
	action := domain.Action{Command: ""}
	text, send := d.Dispatch(action)
	if send {
		t.Errorf("Dispatch(empty command) must return send=false, got send=true, text=%q", text)
	}
}
