package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"gocloud.dev/docstore"
)

// newChatStorage returns chat storage over an empty collection, and the collection for preparing records.
// The collection is in memory, or the collection "test" of the MongoDB database in MONGO_TEST_URL if it is set.
func newChatStorage(t *testing.T) (*ChatStorage, *docstore.Collection) {
	t.Helper()
	store := NewStore("")
	if mongoURL := os.Getenv("MONGO_TEST_URL"); mongoURL != "" {
		store = newMongoCollection(t, mongoURL).store
	}
	collection, err := store.OpenCollection(context.Background(), "test", "chat_id", "bot_type")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = collection.Close() })
	return NewChatStorage(collection), collection
}

// putChats stores the records as they are.
func putChats(t *testing.T, collection *docstore.Collection, chats ...*Chat) {
	t.Helper()
	for _, chat := range chats {
		if err := collection.Put(context.Background(), chat); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// legacyChat returns a record as stored before tokens: without the webhook fields and a revision.
func legacyChat(chatID int64, botType string) map[string]interface{} {
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	return map[string]interface{}{"chat_id": chatID, "bot_type": botType, "created_at": createdAt, "updated_at": createdAt}
}

// putLegacyChats stores records as they were stored before tokens.
func putLegacyChats(t *testing.T, collection *docstore.Collection, chats ...map[string]interface{}) {
	t.Helper()
	for _, chat := range chats {
		if err := collection.Put(context.Background(), chat); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func getChat(t *testing.T, chatStorage *ChatStorage, chatID int64, botType string) *Chat {
	t.Helper()
	chat, err := chatStorage.GetChat(context.Background(), chatID, botType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return chat
}

func TestSaveChat(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a missing record", func(t *testing.T) {
		chatStorage, _ := newChatStorage(t)

		if err := chatStorage.SaveChat(ctx, &Chat{ChatID: 1, BotType: "github"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if chat := getChat(t, chatStorage, 1, "github"); chat == nil || chat.WebhookAuth != WebhookAuthToken {
			t.Errorf("chat = %+v, want a record with %q", chat, WebhookAuthToken)
		}
	})

	t.Run("keeps the webhook state of a record created before tokens", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		record := legacyChat(1, "github")
		putLegacyChats(t, collection, record)

		if err := chatStorage.SaveChat(ctx, &Chat{ChatID: 1, BotType: "github"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		chat := getChat(t, chatStorage, 1, "github")
		updated := chat.UpdatedAt.After(record["updated_at"].(time.Time))
		if chat.WebhookAuth != WebhookAuthAny || chat.WebhookIssuedAt != 0 || !updated {
			t.Errorf("chat = %+v, want an updated record with the webhook state unchanged", chat)
		}
	})
}

func TestGetChatReturnsNilWithoutRecord(t *testing.T) {
	chatStorage, _ := newChatStorage(t)

	if chat := getChat(t, chatStorage, 1, "github"); chat != nil {
		t.Errorf("chat = %+v, want nil", chat)
	}
}

func TestGetOrCreateChat(t *testing.T) {
	ctx := context.Background()

	t.Run("returns a record created before tokens unchanged", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "github"))

		chat, err := chatStorage.GetOrCreateChat(ctx, 1, "github")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat.WebhookAuth != WebhookAuthAny || chat.WebhookIssuedAt != 0 {
			t.Errorf("chat = %+v, want the record created before tokens", chat)
		}
	})

	tests := []struct {
		name         string
		others       []*Chat                  // Records of other bots in the chat
		legacyOthers []map[string]interface{} // Records of other bots in the chat, stored before tokens
		wantAuth     string
	}{
		{name: "creates a token record in a new chat", wantAuth: WebhookAuthToken},
		{
			name:         "creates a legacy record in a chat that used another bot before tokens",
			legacyOthers: []map[string]interface{}{legacyChat(1, "gitlab")},
			wantAuth:     WebhookAuthAny,
		},
		{
			name:     "creates a legacy record in a chat whose other bot has upgraded",
			others:   []*Chat{{ChatID: 1, BotType: "gitlab", WebhookAuth: WebhookAuthUpgraded}},
			wantAuth: WebhookAuthAny,
		},
		{
			name:     "creates a token record in a chat whose other bot started with tokens",
			others:   []*Chat{{ChatID: 1, BotType: "gitlab", WebhookAuth: WebhookAuthToken}},
			wantAuth: WebhookAuthToken,
		},
		{
			name:     "ignores records of other chats",
			others:   []*Chat{{ChatID: 2, BotType: "gitlab", WebhookAuth: WebhookAuthAny}},
			wantAuth: WebhookAuthToken,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatStorage, collection := newChatStorage(t)
			putChats(t, collection, tt.others...)
			putLegacyChats(t, collection, tt.legacyOthers...)

			chat, err := chatStorage.GetOrCreateChat(ctx, 1, "github")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			stored := getChat(t, chatStorage, 1, "github")
			if stored == nil || stored.WebhookAuth != tt.wantAuth || stored.WebhookIssuedAt == 0 {
				t.Errorf("stored chat = %+v, want %q with an issue time", stored, tt.wantAuth)
			}
			if stored != nil && chat.WebhookIssuedAt != stored.WebhookIssuedAt {
				t.Errorf("returned issue time = %d, stored %d", chat.WebhookIssuedAt, stored.WebhookIssuedAt)
			}
		})
	}
}

func TestCreateLegacyChat(t *testing.T) {
	ctx := context.Background()

	t.Run("creates nothing in a chat without legacy history", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putChats(t, collection, &Chat{ChatID: 1, BotType: "gitlab", WebhookAuth: WebhookAuthToken})

		chat, err := chatStorage.CreateLegacyChat(ctx, 1, "github")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat != nil || getChat(t, chatStorage, 1, "github") != nil {
			t.Errorf("chat = %+v, want no record", chat)
		}
	})

	t.Run("creates a legacy record in a chat with legacy history", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "gitlab"))

		chat, err := chatStorage.CreateLegacyChat(ctx, 1, "github")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat == nil || chat.WebhookAuth != WebhookAuthAny {
			t.Errorf("chat = %+v, want a legacy record", chat)
		}
	})

	t.Run("returns a record created concurrently", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "gitlab"))
		putChats(t, collection, &Chat{ChatID: 1, BotType: "github", WebhookAuth: WebhookAuthToken})

		chat, err := chatStorage.CreateLegacyChat(ctx, 1, "github")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat == nil || chat.WebhookAuth != WebhookAuthToken {
			t.Errorf("chat = %+v, want the existing token record", chat)
		}
	})
}

func TestUpgradeWebhookAuth(t *testing.T) {
	ctx := context.Background()

	t.Run("upgrades a record created before tokens and gives it a revision", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "github"))
		chat := getChat(t, chatStorage, 1, "github")
		if chat.DocstoreRevision != nil {
			t.Fatalf("revision = %v, want none before the upgrade", chat.DocstoreRevision)
		}

		if err := chatStorage.UpgradeWebhookAuth(ctx, chat); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		chat = getChat(t, chatStorage, 1, "github")
		if chat.WebhookAuth != WebhookAuthUpgraded || chat.DocstoreRevision == nil {
			t.Errorf("chat = %+v, want an upgraded record with a revision", chat)
		}
	})

	t.Run("ignores a concurrent change of the record", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "github"))
		// The first write gives the record a revision, which the second one changes after the read
		save := func() {
			if err := chatStorage.SaveChat(ctx, &Chat{ChatID: 1, BotType: "github"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		save()
		chat := getChat(t, chatStorage, 1, "github")
		if chat.DocstoreRevision == nil {
			t.Fatal("record has no revision")
		}
		save()

		if err := chatStorage.UpgradeWebhookAuth(ctx, chat); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if chat := getChat(t, chatStorage, 1, "github"); chat.WebhookAuth != WebhookAuthUpgraded {
			t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, WebhookAuthUpgraded)
		}
	})
}

func TestReissueWebhookURL(t *testing.T) {
	ctx := context.Background()

	reissue := func(t *testing.T, chatStorage *ChatStorage, chatID int64, botType string) int64 {
		t.Helper()
		issuedAt, err := chatStorage.ReissueWebhookURL(ctx, chatID, botType)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chat := getChat(t, chatStorage, chatID, botType); chat == nil || chat.WebhookIssuedAt != issuedAt {
			t.Fatalf("chat = %+v, want a record with the returned issue time %d", chat, issuedAt)
		}
		return issuedAt
	}

	t.Run("creates a record in a chat without one", func(t *testing.T) {
		chatStorage, _ := newChatStorage(t)

		issuedAt := reissue(t, chatStorage, 1, "github")

		// The record is created with the current time, which the reissue increments within the same second
		if offset := issuedAt - time.Now().Unix(); offset < -60 || offset > 1 {
			t.Errorf("issue time = %d, want about the current time", issuedAt)
		}
		if chat := getChat(t, chatStorage, 1, "github"); chat.WebhookAuth != WebhookAuthToken {
			t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, WebhookAuthToken)
		}
	})

	t.Run("issues a later time than the previous one within the same second", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		previous := time.Now().Unix() + 100
		putChats(t, collection, &Chat{ChatID: 1, BotType: "github", WebhookAuth: WebhookAuthToken, WebhookIssuedAt: previous})

		if issuedAt := reissue(t, chatStorage, 1, "github"); issuedAt != previous+1 {
			t.Errorf("issue time = %d, want %d", issuedAt, previous+1)
		}
		if issuedAt := reissue(t, chatStorage, 1, "github"); issuedAt != previous+2 {
			t.Errorf("second issue time = %d, want %d", issuedAt, previous+2)
		}
	})

	t.Run("upgrades a record created before tokens", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "github"))

		if issuedAt := reissue(t, chatStorage, 1, "github"); issuedAt == 0 {
			t.Error("issue time = 0, want the current time")
		}
		if chat := getChat(t, chatStorage, 1, "github"); chat.WebhookAuth != WebhookAuthUpgraded {
			t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, WebhookAuthUpgraded)
		}
	})

	t.Run("creates an upgraded record in a chat with legacy history", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putChats(t, collection, &Chat{ChatID: 1, BotType: "gitlab", WebhookAuth: WebhookAuthAny})

		reissue(t, chatStorage, 1, "github")

		if chat := getChat(t, chatStorage, 1, "github"); chat.WebhookAuth != WebhookAuthUpgraded {
			t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, WebhookAuthUpgraded)
		}
	})

	t.Run("keeps a record that accepts only URLs with a token", func(t *testing.T) {
		for _, auth := range []string{WebhookAuthToken, WebhookAuthUpgraded} {
			chatStorage, collection := newChatStorage(t)
			putChats(t, collection, &Chat{ChatID: 1, BotType: "github", WebhookAuth: auth})

			reissue(t, chatStorage, 1, "github")

			if chat := getChat(t, chatStorage, 1, "github"); chat.WebhookAuth != auth {
				t.Errorf("WebhookAuth = %q, want %q", chat.WebhookAuth, auth)
			}
		}
	})
}

func TestClaimLegacyWarning(t *testing.T) {
	ctx := context.Background()
	const interval = 24 * time.Hour

	claim := func(t *testing.T, chatStorage *ChatStorage, chat *Chat) bool {
		t.Helper()
		claimed, err := chatStorage.ClaimLegacyWarning(ctx, chat, interval)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return claimed
	}

	t.Run("claims once per interval", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putChats(t, collection, &Chat{ChatID: 1, BotType: "github", WebhookAuth: WebhookAuthUpgraded})

		if !claim(t, chatStorage, getChat(t, chatStorage, 1, "github")) {
			t.Fatal("first claim failed")
		}
		chat := getChat(t, chatStorage, 1, "github")
		if time.Since(chat.LegacyWarnedAt) > time.Minute {
			t.Errorf("LegacyWarnedAt = %v, want the time of the claim", chat.LegacyWarnedAt)
		}
		if claim(t, chatStorage, chat) {
			t.Error("second claim within the interval succeeded")
		}
	})

	t.Run("claims after the interval", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putChats(t, collection, &Chat{
			ChatID:         1,
			BotType:        "github",
			WebhookAuth:    WebhookAuthUpgraded,
			LegacyWarnedAt: time.Now().Add(-interval - time.Minute),
		})

		if !claim(t, chatStorage, getChat(t, chatStorage, 1, "github")) {
			t.Error("claim after the interval failed")
		}
	})

	t.Run("only one of concurrent readers of an upgraded record created before tokens claims", func(t *testing.T) {
		chatStorage, collection := newChatStorage(t)
		putLegacyChats(t, collection, legacyChat(1, "github"))
		if err := chatStorage.UpgradeWebhookAuth(ctx, getChat(t, chatStorage, 1, "github")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		first := getChat(t, chatStorage, 1, "github")
		second := getChat(t, chatStorage, 1, "github")

		if !claim(t, chatStorage, first) {
			t.Fatal("first claim failed")
		}
		if claim(t, chatStorage, second) {
			t.Error("claim with a record read before the first claim succeeded")
		}
	})
}
