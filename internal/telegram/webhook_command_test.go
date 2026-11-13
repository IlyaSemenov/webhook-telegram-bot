package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"slices"
	"strconv"
	"testing"

	"webhook-telegram-bot/internal/storage"

	"github.com/go-telegram/bot/models"
	"gocloud.dev/docstore"
)

// failingChatStore is chat storage that fails to get, create, or reissue the record of a chat.
type failingChatStore struct {
	*storage.ChatStorage
}

func (s failingChatStore) GetOrCreateChat(context.Context, int64, string) (*storage.Chat, error) {
	return nil, errors.New("storage failure")
}

func (s failingChatStore) ReissueWebhookURL(context.Context, int64, string) (int64, error) {
	return 0, errors.New("storage failure")
}

// newWebhookCommand returns a service from newWebhookService with the /webhook command of the source Test,
// the collection for preparing records, and the fake Telegram Bot API that the service uses.
func newWebhookCommand(t *testing.T) (*TelegramService, *docstore.Collection, *telegramServer) {
	t.Helper()
	service, collection, server := newWebhookService(t)
	service.RegisterWebhookCommand("Test", "<b>Setup</b>")
	return service, collection, server
}

// processUpdate handles the update as the bot handles a Telegram update.
func processUpdate(service *TelegramService, update *models.Update) {
	service.bot.ProcessUpdate(context.Background(), update)
}

// newCommandUpdate returns the update with the command sent to the chat.
func newCommandUpdate(chatID int64, command string) *models.Update {
	return &models.Update{Message: &models.Message{
		Chat:     models.Chat{ID: chatID},
		Text:     command,
		Entities: []models.MessageEntity{{Type: models.MessageEntityTypeBotCommand, Offset: 0, Length: len(command)}},
	}}
}

// pressedMessageID is the ID of the message with the button pressed in newButtonUpdate.
const pressedMessageID = 7

// newButtonUpdate returns the update with a press of the button with the callback data under a message in the chat.
func newButtonUpdate(chatID int64, data string) *models.Update {
	return &models.Update{CallbackQuery: &models.CallbackQuery{
		ID:   "query",
		Data: data,
		Message: models.MaybeInaccessibleMessage{
			Type:    models.MaybeInaccessibleMessageTypeMessage,
			Message: &models.Message{ID: pressedMessageID, Chat: models.Chat{ID: chatID}},
		},
	}}
}

// getButtonData returns the callback data of the buttons in the request, or nil if the request has none.
func getButtonData(t *testing.T, request telegramRequest) []string {
	t.Helper()
	replyMarkup := request.form.Get("reply_markup")
	if replyMarkup == "" {
		return nil
	}
	var keyboard models.InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(replyMarkup), &keyboard); err != nil {
		t.Fatalf("failed to parse reply markup %q: %v", replyMarkup, err)
	}
	var data []string
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			data = append(data, button.CallbackData)
		}
	}
	return data
}

// getMethods returns the methods of the requests.
func getMethods(requests []telegramRequest) []string {
	var methods []string
	for _, request := range requests {
		methods = append(methods, request.method)
	}
	return methods
}

// checkRequests checks that the requests call the methods in order.
func checkRequests(t *testing.T, server *telegramServer, methods ...string) {
	t.Helper()
	if got := getMethods(server.requests); !slices.Equal(got, methods) {
		t.Fatalf("methods = %q, want %q", got, methods)
	}
}

// checkButtonAnswer checks that the request answers the button press with the alert, or without one if alertText is empty.
func checkButtonAnswer(t *testing.T, request telegramRequest, alertText string) {
	t.Helper()
	if got := request.form.Get("text"); got != alertText {
		t.Errorf("alert = %q, want %q", got, alertText)
	}
	if got, want := request.form.Get("show_alert") == "true", alertText != ""; got != want {
		t.Errorf("show_alert = %v, want %v", got, want)
	}
}

// checkButtons checks that the request sets the buttons with the callback data.
func checkButtons(t *testing.T, request telegramRequest, data ...string) {
	t.Helper()
	if got := getButtonData(t, request); !slices.Equal(got, data) {
		t.Errorf("buttons = %q, want %q", got, data)
	}
}

func TestWebhookCommand(t *testing.T) {
	t.Run("replies with the URL, the setup instructions, and the reissue button", func(t *testing.T) {
		service, _, server := newWebhookCommand(t)
		service.baseURL = "https://bot.example.com/a&b"

		processUpdate(service, newCommandUpdate(-100, "/webhook"))

		webhookURL, err := service.getChatWebhookURL(context.Background(), -100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkRequests(t, server, "sendMessage")
		want := "🔗 <b>Your Test Webhook URL</b>\n\n<code>" + html.EscapeString(webhookURL) + "</code>\n\n" + webhookURLNotice + "\n\n<b>Setup</b>"
		if got := server.requests[0].form.Get("text"); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		checkButtons(t, server.requests[0], reissueData)
	})

	t.Run("replies with an error message if the URL cannot be issued", func(t *testing.T) {
		service, collection, server := newWebhookCommand(t)
		service.chatStorage = failingChatStore{storage.NewChatStorage(collection)}

		processUpdate(service, newCommandUpdate(-100, "/webhook"))

		checkRequests(t, server, "sendMessage")
		if got := server.requests[0].form.Get("text"); got != webhookURLErrorText {
			t.Errorf("text = %q, want %q", got, webhookURLErrorText)
		}
		checkButtons(t, server.requests[0])
	})
}

func TestWebhookCommandReissue(t *testing.T) {
	ctx := context.Background()

	// newCommandWithChat returns newWebhookCommand with the record of a chat whose URL was issued at the time 100.
	newCommandWithChat := func(t *testing.T) (*TelegramService, *docstore.Collection, *telegramServer) {
		t.Helper()
		service, collection, server := newWebhookCommand(t)
		chat := &storage.Chat{ChatID: -100, BotType: "test", WebhookAuth: storage.WebhookAuthToken, WebhookIssuedAt: 100}
		if err := collection.Put(ctx, chat); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return service, collection, server
	}

	// getIssuedAt returns the stored issue time of the URL of the chat.
	getIssuedAt := func(t *testing.T, service *TelegramService) int64 {
		t.Helper()
		chat, err := service.chatStorage.GetChat(ctx, -100, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return chat.WebhookIssuedAt
	}

	// getReissuedText returns the reply with the URL of the chat issued at issuedAt after a reissue.
	getReissuedText := func(service *TelegramService, issuedAt int64) string {
		command := &webhookCommand{service: service, sourceName: "Test", setupText: "<b>Setup</b>"}
		return reissuedText + "\n\n" + command.formatText(service.createWebhookURL(-100, issuedAt))
	}

	t.Run("warns and asks to confirm", func(t *testing.T) {
		service, _, server := newCommandWithChat(t)

		processUpdate(service, newButtonUpdate(-100, reissueData))

		checkRequests(t, server, "answerCallbackQuery", "editMessageReplyMarkup")
		checkButtonAnswer(t, server.requests[0], reissueWarningText)
		checkButtons(t, server.requests[1], confirmReissueData, cancelReissueData)
		if issuedAt := getIssuedAt(t, service); issuedAt != 100 {
			t.Errorf("issue time = %d, want the previous 100", issuedAt)
		}
	})

	t.Run("restores the reissue button on cancel", func(t *testing.T) {
		service, _, server := newCommandWithChat(t)

		processUpdate(service, newButtonUpdate(-100, cancelReissueData))

		checkRequests(t, server, "answerCallbackQuery", "editMessageReplyMarkup")
		checkButtonAnswer(t, server.requests[0], "")
		checkButtons(t, server.requests[1], reissueData)
		if issuedAt := getIssuedAt(t, service); issuedAt != 100 {
			t.Errorf("issue time = %d, want the previous 100", issuedAt)
		}
	})

	t.Run("reissues the URL on confirmation and shows it in the message", func(t *testing.T) {
		service, _, server := newCommandWithChat(t)

		processUpdate(service, newButtonUpdate(-100, confirmReissueData))

		issuedAt := getIssuedAt(t, service)
		if issuedAt <= 100 {
			t.Fatalf("issue time = %d, want later than the previous 100", issuedAt)
		}
		checkRequests(t, server, "answerCallbackQuery", "editMessageText")
		checkButtonAnswer(t, server.requests[0], "")
		edit := server.requests[1]
		if got, want := edit.form.Get("message_id"), strconv.Itoa(pressedMessageID); got != want {
			t.Errorf("edited message = %s, want %s", got, want)
		}
		if got, want := edit.form.Get("text"), getReissuedText(service, issuedAt); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		checkButtons(t, edit, reissueData)
	})

	t.Run("sends the reissued URL in a new message if the message cannot be edited", func(t *testing.T) {
		service, _, server := newCommandWithChat(t)
		server.failMethod = "editMessageText"

		processUpdate(service, newButtonUpdate(-100, confirmReissueData))

		checkRequests(t, server, "answerCallbackQuery", "editMessageText", "sendMessage")
		send := server.requests[2]
		if got, want := send.form.Get("text"), getReissuedText(service, getIssuedAt(t, service)); got != want {
			t.Errorf("text = %q, want %q", got, want)
		}
		checkButtons(t, send, reissueData)
	})

	t.Run("reissues the URL from a button under an inaccessible message", func(t *testing.T) {
		service, _, server := newCommandWithChat(t)
		server.failMethod = "editMessageText"
		update := newButtonUpdate(-100, confirmReissueData)
		update.CallbackQuery.Message = models.MaybeInaccessibleMessage{
			Type:                models.MaybeInaccessibleMessageTypeInaccessibleMessage,
			InaccessibleMessage: &models.InaccessibleMessage{Chat: models.Chat{ID: -100}, MessageID: pressedMessageID},
		}

		processUpdate(service, update)

		if issuedAt := getIssuedAt(t, service); issuedAt <= 100 {
			t.Fatalf("issue time = %d, want later than the previous 100", issuedAt)
		}
		checkRequests(t, server, "answerCallbackQuery", "editMessageText", "sendMessage")
		edit, send := server.requests[1], server.requests[2]
		if got, want := edit.form.Get("message_id"), strconv.Itoa(pressedMessageID); got != want {
			t.Errorf("edited message = %s, want %s", got, want)
		}
		for _, request := range []telegramRequest{edit, send} {
			if got := request.form.Get("chat_id"); got != "-100" {
				t.Errorf("%s chat = %s, want -100", request.method, got)
			}
		}
	})

	t.Run("shows an alert and restores the reissue button if the URL cannot be reissued", func(t *testing.T) {
		service, collection, server := newCommandWithChat(t)
		service.chatStorage = failingChatStore{storage.NewChatStorage(collection)}

		processUpdate(service, newButtonUpdate(-100, confirmReissueData))

		checkRequests(t, server, "answerCallbackQuery", "editMessageReplyMarkup")
		checkButtonAnswer(t, server.requests[0], reissueErrorText)
		checkButtons(t, server.requests[1], reissueData)
	})
}
