package telegram

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
)

const testBotToken = "123456:test-token"

func TestRedactBotToken(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "request failure",
			err:  errors.New(`error do request for method sendMessage, Post "https://api.telegram.org/bot123456:test-token/sendMessage": dial tcp: i/o timeout`),
			want: `error do request for method sendMessage, Post "https://api.telegram.org/bot***/sendMessage": dial tcp: i/o timeout`,
		},
		{
			name: "URL without method",
			err:  errors.New(`parse "https://api.telegram.org/bot123456:test-token": invalid URL`),
			want: `parse "https://api.telegram.org/bot***": invalid URL`,
		},
		{
			name: "token with a line break",
			err:  errors.New(`parse "https://api.telegram.org/bot123456:test-token\n654321:other-token/getMe": invalid control character`),
			want: `parse "https://api.telegram.org/bot***/getMe": invalid control character`,
		},
		{
			name: "error without a token",
			err:  errors.New("Bad Request: chat not found"),
			want: "Bad Request: chat not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactBotToken(tt.err).Error(); got != tt.want {
				t.Errorf("redactBotToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactBotTokenKeepsNilAndErrorsWithoutToken(t *testing.T) {
	if err := redactBotToken(nil); err != nil {
		t.Errorf("redactBotToken(nil) = %v, want nil", err)
	}

	original := errors.New("Bad Request: chat not found")
	if err := redactBotToken(original); err != original {
		t.Errorf("redactBotToken() = %v, want the original error", err)
	}
}

func TestTelegramServiceErrorsDoNotContainBotToken(t *testing.T) {
	// Nothing listens on this address, so every request fails with an error that quotes the request URL
	telegramBot, err := bot.New(testBotToken, bot.WithSkipGetMe(), bot.WithServerURL("http://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	service := &TelegramService{botId: "test", bot: telegramBot}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "InitBot", call: service.InitBot},
		{name: "SetWebhook", call: func() error { return service.SetWebhook("https://example.com") }},
		{name: "SendMessage", call: func() error { return service.SendMessage(1, "text") }},
		{name: "UpdateMessage", call: func() error { return service.UpdateMessage(1, 1, "text") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), testBotToken) {
				t.Errorf("error contains the bot token: %v", err)
			}
		})
	}
}

func TestTelegramServiceLogsDoNotContainBotToken(t *testing.T) {
	telegramBot, err := bot.New(testBotToken, bot.WithSkipGetMe(), bot.WithServerURL("http://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	service := &TelegramService{botId: "test", bot: telegramBot}

	var output bytes.Buffer
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	tests := []struct {
		name string
		call func()
	}{
		{name: "SetCommands", call: func() { service.SetCommands(nil) }},
		{name: "SetWhatCanThisBotDo", call: func() { service.SetWhatCanThisBotDo("description") }},
		{name: "SetProfileDescription", call: func() { service.SetProfileDescription("description") }},
		{name: "SendMessageOrLogError", call: func() { service.SendMessageOrLogError(1, "text") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Reset()
			tt.call()
			if !strings.Contains(output.String(), "/bot***") {
				t.Errorf("log does not contain the redacted error: %q", output.String())
			}
			if strings.Contains(output.String(), testBotToken) {
				t.Errorf("log contains the bot token: %q", output.String())
			}
		})
	}
}

func TestIsBotBlockedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "bot was blocked by the user",
			err:  errors.New("Forbidden: bot was blocked by the user"),
			want: true,
		},
		{
			name: "user is deactivated",
			err:  errors.New("Forbidden: user is deactivated"),
			want: true,
		},
		{
			name: "chat not found",
			err:  errors.New("Bad Request: chat not found"),
			want: true,
		},
		{
			name: "bot is not a member",
			err:  errors.New("Forbidden: bot is not a member of the supergroup chat"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("timeout: connection reset"),
			want: false,
		},
		{
			name: "partial match should not trigger",
			err:  errors.New("the bot was not blocked"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotBlockedError(tt.err)
			if got != tt.want {
				t.Errorf("isBotBlockedError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
