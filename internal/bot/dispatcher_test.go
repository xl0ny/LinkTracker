package bot_test

import (
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	d := bot.NewDispatcher(commands.All())
	action := model.Action{Command: "start"}
	text, send := d.Dispatch(action, (*tgbotapi.BotAPI)(nil))
	if !send {
		t.Fatal("Dispatch(start) must return send=true")
	}
	want := "Добро пожаловать! Используйте /help"
	if !strings.Contains(text, want) {
		t.Errorf("Dispatch(start) = %q, want substring %q", text, want)
	}
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	d := bot.NewDispatcher(commands.All())
	action := model.Action{Command: "help"}
	text, send := d.Dispatch(action, (*tgbotapi.BotAPI)(nil))
	if !send {
		t.Fatal("Dispatch(help) must return send=true")
	}
	if !strings.Contains(text, "/start") || !strings.Contains(text, "/help") {
		t.Errorf("Dispatch(help) must contain /start and /help, got %q", text)
	}
}

// Негативный сценарий (blackbox): при получении неизвестной команды бот отвечает сообщением об ошибке.
func TestDispatcher_Dispatch_unknownCommand_returnsErrorMessage(t *testing.T) {
	d := bot.NewDispatcher(commands.All())
	action := model.Action{Command: "unknowncommand"}
	text, send := d.Dispatch(action, (*tgbotapi.BotAPI)(nil))
	if !send {
		t.Fatal("Dispatch(unknown) must return send=true (error message)")
	}
	if !strings.Contains(text, "Неизвестная команда") || !strings.Contains(text, "/help") {
		t.Errorf("Dispatch(unknown) must mention unknown command and /help, got %q", text)
	}
}

func TestDispatcher_Dispatch_emptyCommand_noResponse(t *testing.T) {
	d := bot.NewDispatcher(commands.All())
	action := model.Action{Command: ""}
	text, send := d.Dispatch(action, (*tgbotapi.BotAPI)(nil))
	if send {
		t.Errorf("Dispatch(empty command) must return send=false, got send=true, text=%q", text)
	}
}
