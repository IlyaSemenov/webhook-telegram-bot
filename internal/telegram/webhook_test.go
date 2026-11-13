package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webhook-telegram-bot/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/gorilla/mux"
	"gocloud.dev/docstore"
)

func TestReadWebhookBody(t *testing.T) {
	t.Run("returns a body of the maximum size", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxWebhookBodySize)))
		response := httptest.NewRecorder()

		body, ok := ReadWebhookBody(response, request)

		if !ok || len(body) != maxWebhookBodySize {
			t.Errorf("ok = %v, length = %d", ok, len(body))
		}
	})

	t.Run("rejects a larger body", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxWebhookBodySize+1)))
		response := httptest.NewRecorder()

		if _, ok := ReadWebhookBody(response, request); ok {
			t.Error("ok = true, want false")
		}
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
		}
	})
}

// telegramServer is a fake Telegram Bot API that records the texts of sent messages.
type telegramServer struct {
	*httptest.Server
	sentTexts []string
}

func newTelegramServer(t *testing.T) *telegramServer {
	t.Helper()
	server := &telegramServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("failed to parse Telegram request: %v", err)
		}
		server.sentTexts = append(server.sentTexts, r.FormValue("text"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"group"}}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// newWebhookService returns a service with chat storage over a new in-memory collection that sends messages to a fake Telegram Bot API,
// and the collection for preparing records.
func newWebhookService(t *testing.T) (*TelegramService, *docstore.Collection, *telegramServer) {
	t.Helper()
	server := newTelegramServer(t)
	telegramBot, err := bot.New(testBotToken, bot.WithSkipGetMe(), bot.WithServerURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collection, err := storage.OpenChatCollection(context.Background(), storage.NewStore(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = collection.Close() })
	service := &TelegramService{
		botId:       "test",
		bot:         telegramBot,
		chatStorage: storage.NewChatStorage(collection),
		baseURL:     "https://bot.example.com",
		secretKey:   "secret",
	}
	return service, collection, server
}

// failingUpgradeStore is chat storage that fails to make a chat accept only URLs with a token.
type failingUpgradeStore struct {
	*storage.ChatStorage
}

func (s failingUpgradeStore) UpgradeWebhookAuth(context.Context, *storage.Chat) error {
	return errors.New("storage failure")
}

func TestGetChatWebhookURL(t *testing.T) {
	ctx := context.Background()

	t.Run("issues a URL to a chat without a record and keeps it", func(t *testing.T) {
		service, _, _ := newWebhookService(t)

		webhookURL, err := service.GetChatWebhookURL(ctx, -100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		chat, err := service.chatStorage.GetChat(ctx, -100, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat == nil || chat.WebhookIssuedAt == 0 {
			t.Fatalf("chat = %+v, want a record with an issue time", chat)
		}
		want := "https://bot.example.com/test/-100/" + createWebhookToken("secret", "test", -100, chat.WebhookIssuedAt)
		if webhookURL != want {
			t.Errorf("URL = %q, want %q", webhookURL, want)
		}

		again, err := service.GetChatWebhookURL(ctx, -100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if again != webhookURL {
			t.Errorf("second URL = %q, want the first %q", again, webhookURL)
		}
	})

	t.Run("uses issue time zero for a record created before tokens", func(t *testing.T) {
		service, collection, _ := newWebhookService(t)
		if err := collection.Put(ctx, &storage.Chat{ChatID: -100, BotType: "test"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		webhookURL, err := service.GetChatWebhookURL(ctx, -100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "https://bot.example.com/test/-100/" + createWebhookToken("secret", "test", -100, 0); webhookURL != want {
			t.Errorf("URL = %q, want %q", webhookURL, want)
		}
	})
}

func TestCreateWebhookTokenDependsOnEveryInput(t *testing.T) {
	token := createWebhookToken("secret", "test", 1, 100)
	others := []string{
		createWebhookToken("other", "test", 1, 100),
		createWebhookToken("secret", "other", 1, 100),
		createWebhookToken("secret", "test", 2, 100),
		createWebhookToken("secret", "test", 1, 101),
	}
	for _, other := range others {
		if other == token {
			t.Errorf("token %q does not depend on every input", token)
		}
	}
}

func TestAuthorizeWebhook(t *testing.T) {
	const chatID = -100
	const issuedAt = 1000
	validToken := createWebhookToken("secret", "test", chatID, issuedAt)
	// otherBot is the record of another bot in the chat, which shows whether the chat used bots before tokens
	otherBot := func(auth string) *storage.Chat {
		return &storage.Chat{ChatID: chatID, BotType: "other", WebhookAuth: auth}
	}

	tests := []struct {
		name        string
		chats       []*storage.Chat // Records before the request; ChatID and BotType of the bot's own record are filled in
		legacyChat  bool            // Whether the bot's own record is stored as before tokens: without the webhook fields and a revision
		failUpgrade bool
		path        string
		wantCode    int
		wantAuth    string // WebhookAuth of the bot's record after the request, or "-" for no record
		wantWarning bool
	}{
		{
			name:     "valid token switches a legacy chat to tokens",
			chats:    []*storage.Chat{{WebhookIssuedAt: issuedAt}},
			path:     "/test/-100/" + validToken,
			wantCode: http.StatusOK,
			wantAuth: storage.WebhookAuthUpgraded,
		},
		{
			name:       "token of issue time zero for a record created before tokens",
			legacyChat: true,
			path:       "/test/-100/" + createWebhookToken("secret", "test", chatID, 0),
			wantCode:   http.StatusOK,
			wantAuth:   storage.WebhookAuthUpgraded,
		},
		{
			name:     "valid token for a token chat",
			chats:    []*storage.Chat{{WebhookAuth: storage.WebhookAuthToken, WebhookIssuedAt: issuedAt}},
			path:     "/test/-100/" + validToken,
			wantCode: http.StatusOK,
			wantAuth: storage.WebhookAuthToken,
		},
		{
			name:        "failure to save the upgrade rejects the request",
			chats:       []*storage.Chat{{WebhookIssuedAt: issuedAt}},
			failUpgrade: true,
			path:        "/test/-100/" + validToken,
			wantCode:    http.StatusInternalServerError,
			wantAuth:    storage.WebhookAuthAny,
		},
		{
			name:     "token of another issue time",
			chats:    []*storage.Chat{{WebhookIssuedAt: issuedAt}},
			path:     "/test/-100/" + createWebhookToken("secret", "test", chatID, issuedAt+1),
			wantCode: http.StatusUnauthorized,
			wantAuth: storage.WebhookAuthAny,
		},
		{
			name:     "token of another chat",
			chats:    []*storage.Chat{{WebhookIssuedAt: issuedAt}},
			path:     "/test/-100/" + createWebhookToken("secret", "test", chatID-1, issuedAt),
			wantCode: http.StatusUnauthorized,
			wantAuth: storage.WebhookAuthAny,
		},
		{
			name:     "token for a chat without a record",
			path:     "/test/-100/" + validToken,
			wantCode: http.StatusUnauthorized,
			wantAuth: "-",
		},
		{
			name:       "no token for a record created before tokens",
			legacyChat: true,
			path:       "/test/-100",
			wantCode:   http.StatusOK,
			wantAuth:   storage.WebhookAuthAny,
		},
		{
			name:     "no token for a chat without records",
			path:     "/test/-100",
			wantCode: http.StatusUnauthorized,
			wantAuth: "-",
		},
		{
			name:     "no token for a chat that used another bot before tokens creates a legacy record",
			chats:    []*storage.Chat{otherBot(storage.WebhookAuthAny)},
			path:     "/test/-100",
			wantCode: http.StatusOK,
			wantAuth: storage.WebhookAuthAny,
		},
		{
			name:     "no token for a chat whose other bot started with tokens",
			chats:    []*storage.Chat{otherBot(storage.WebhookAuthToken)},
			path:     "/test/-100",
			wantCode: http.StatusUnauthorized,
			wantAuth: "-",
		},
		{
			name:     "no token for a token chat",
			chats:    []*storage.Chat{{WebhookAuth: storage.WebhookAuthToken}},
			path:     "/test/-100",
			wantCode: http.StatusUnauthorized,
			wantAuth: storage.WebhookAuthToken,
		},
		{
			name:        "no token for an upgraded chat warns the chat",
			chats:       []*storage.Chat{{WebhookAuth: storage.WebhookAuthUpgraded}},
			path:        "/test/-100",
			wantCode:    http.StatusUnauthorized,
			wantAuth:    storage.WebhookAuthUpgraded,
			wantWarning: true,
		},
		{
			name:     "no token for a recently warned upgraded chat",
			chats:    []*storage.Chat{{WebhookAuth: storage.WebhookAuthUpgraded, LegacyWarnedAt: time.Now().Add(-time.Hour)}},
			path:     "/test/-100",
			wantCode: http.StatusUnauthorized,
			wantAuth: storage.WebhookAuthUpgraded,
		},
		{
			name:     "invalid chat ID",
			path:     "/test/abc",
			wantCode: http.StatusBadRequest,
			wantAuth: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, collection, server := newWebhookService(t)
			ctx := context.Background()
			for _, chat := range tt.chats {
				if chat.BotType == "" {
					chat.ChatID, chat.BotType = chatID, "test"
				}
				if err := collection.Put(ctx, chat); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if tt.legacyChat {
				record := map[string]interface{}{"chat_id": int64(chatID), "bot_type": "test", "created_at": time.Now(), "updated_at": time.Now()}
				if err := collection.Put(ctx, record); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if tt.failUpgrade {
				service.chatStorage = failingUpgradeStore{storage.NewChatStorage(collection)}
			}

			called := false
			router := mux.NewRouter()
			handler := service.AuthorizeWebhook(func(w http.ResponseWriter, r *http.Request) { called = true })
			router.HandleFunc("/test/{chatID}/{token}", handler)
			router.HandleFunc("/test/{chatID}", handler)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, tt.path, nil))

			if called != (tt.wantCode == http.StatusOK) {
				t.Errorf("next called = %v, want %v", called, !called)
			}
			if response.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", response.Code, tt.wantCode)
			}
			if gotWarning := len(server.sentTexts) > 0; gotWarning != tt.wantWarning {
				t.Errorf("warning sent = %v, want %v (texts: %q)", gotWarning, tt.wantWarning, server.sentTexts)
			}
			if tt.wantWarning && server.sentTexts[0] != legacyWarningText {
				t.Errorf("warning = %q, want %q", server.sentTexts[0], legacyWarningText)
			}

			chat, err := service.chatStorage.GetChat(ctx, chatID, "test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch {
			case tt.wantAuth == "-":
				if chat != nil {
					t.Errorf("chat = %+v, want no record", chat)
				}
			case chat == nil:
				t.Errorf("no record, want WebhookAuth %q", tt.wantAuth)
			case chat.WebhookAuth != tt.wantAuth:
				t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, tt.wantAuth)
			}
		})
	}
}
