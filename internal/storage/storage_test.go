package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gocloud.dev/docstore"
)

// testDoc is a document of a collection keyed by two fields, like chats.
type testDoc struct {
	ChatID  int64  `docstore:"chat_id"`
	BotType string `docstore:"bot_type"`
	Value   string `docstore:"value"`
}

// testCompositeKeys checks that documents differing only in the sort key are separate, on a collection keyed by chat_id and bot_type.
func testCompositeKeys(t *testing.T, collection *docstore.Collection) {
	t.Helper()
	ctx := context.Background()

	for _, doc := range []*testDoc{{1, "github", "a"}, {1, "gitlab", "b"}} {
		if err := collection.Put(ctx, doc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := collection.Delete(ctx, &testDoc{ChatID: 1, BotType: "github"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := &testDoc{ChatID: 1, BotType: "gitlab"}
	if err := collection.Get(ctx, doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Value != "b" {
		t.Errorf("value = %q, want %q", doc.Value, "b")
	}
	if err := collection.Get(ctx, &testDoc{ChatID: 1, BotType: "github"}); err == nil {
		t.Error("deleted document is still found")
	}
}

func TestCreateCompositeKeyFunc(t *testing.T) {
	keyFunc := createCompositeKeyFunc("chat_id", "bot_type")

	fromStruct := keyFunc(&testDoc{ChatID: -100, BotType: "github"})
	fromMap := keyFunc(map[string]interface{}{"chat_id": int64(-100), "bot_type": "github"})

	if fromStruct != "-100:github" || fromMap != fromStruct {
		t.Errorf("keys = %v and %v, want -100:github", fromStruct, fromMap)
	}
}

func TestOpenCollectionInMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("keys by both fields", func(t *testing.T) {
		collection, err := NewStore("").OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		testCompositeKeys(t, collection)
	})

	t.Run("opens a separate collection each time", func(t *testing.T) {
		store := NewStore("")
		first, err := store.OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := store.OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := first.Put(ctx, &testDoc{ChatID: 1, BotType: "github"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := second.Get(ctx, &testDoc{ChatID: 1, BotType: "github"}); err == nil {
			t.Error("document of another collection is found")
		}
	})
}

// TestOpenCollectionMongo runs against the MongoDB database in MONGO_TEST_URL, dropping its collection "test", and is skipped without it.
func TestOpenCollectionMongo(t *testing.T) {
	mongoURL := os.Getenv("MONGO_TEST_URL")
	if mongoURL == "" {
		t.Skip("MONGO_TEST_URL is not set")
	}
	ctx := context.Background()

	t.Run("keys by both fields", func(t *testing.T) {
		m := newMongoCollection(t, mongoURL)
		collection, err := m.store.OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		testCompositeKeys(t, collection)
	})

	t.Run("moves documents keyed by the first field", func(t *testing.T) {
		m := newMongoCollection(t, mongoURL)
		createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		m.insert(t,
			// A document stored before composite keys
			bson.M{"_id": int64(-100), "bot_type": "github", "created_at": createdAt},
			// A stale copy of a document already moved, which an old version recreated
			bson.M{"_id": int64(-200), "bot_type": "github", "value": "stale"},
			bson.M{"_id": "-200:github", "chat_id": int64(-200), "bot_type": "github", "value": "moved"},
		)

		// Opening twice checks that the migration is idempotent
		for range 2 {
			if _, err := m.store.OpenCollection(ctx, "test", "chat_id", "bot_type"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		var docs []bson.M
		cursor, err := m.collection.Find(ctx, bson.M{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := cursor.All(ctx, &docs); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(docs) != 2 {
			t.Fatalf("documents = %v, want 2", docs)
		}

		collection, err := m.store.OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var moved struct {
			ChatID    int64     `docstore:"chat_id"`
			BotType   string    `docstore:"bot_type"`
			CreatedAt time.Time `docstore:"created_at"`
		}
		moved.ChatID, moved.BotType = -100, "github"
		if err := collection.Get(ctx, &moved); err != nil {
			t.Fatalf("moved document is not found: %v", err)
		}
		if !moved.CreatedAt.Equal(createdAt) {
			t.Errorf("created_at = %v, want %v", moved.CreatedAt, createdAt)
		}

		kept := &testDoc{ChatID: -200, BotType: "github"}
		if err := collection.Get(ctx, kept); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kept.Value != "moved" {
			t.Errorf("value = %q, want the already moved document", kept.Value)
		}
	})

	t.Run("keeps a document that fails to move", func(t *testing.T) {
		m := newMongoCollection(t, mongoURL)
		// The validator rejects documents with a composite key, so the insert of the moved document fails
		database := m.collection.Database()
		validator := bson.M{"_id": bson.M{"$not": bson.M{"$type": "string"}}}
		if err := database.CreateCollection(ctx, "test", options.CreateCollection().SetValidator(validator)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m.insert(t, bson.M{"_id": int64(-100), "bot_type": "github", "value": "old"})

		if _, err := m.store.OpenCollection(ctx, "test", "chat_id", "bot_type"); err == nil {
			t.Fatal("expected an error")
		}
		var old bson.M
		if err := m.collection.FindOne(ctx, bson.M{"_id": int64(-100)}).Decode(&old); err != nil {
			t.Fatalf("document that failed to move is lost: %v", err)
		}

		if err := database.RunCommand(ctx, bson.D{{Key: "collMod", Value: "test"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		collection, err := m.store.OpenCollection(ctx, "test", "chat_id", "bot_type")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		moved := &testDoc{ChatID: -100, BotType: "github"}
		if err := collection.Get(ctx, moved); err != nil {
			t.Fatalf("document is not moved on the next open: %v", err)
		}
		if moved.Value != "old" {
			t.Errorf("value = %q, want %q", moved.Value, "old")
		}
	})
}

// mongoCollection is a store and the raw MongoDB collection "test" that its collections use.
type mongoCollection struct {
	store      *Store
	collection *mongo.Collection
}

// newMongoCollection returns a store for mongoURL and its emptied raw collection "test".
func newMongoCollection(t *testing.T, mongoURL string) *mongoCollection {
	t.Helper()
	store := NewStore(mongoURL)
	database, err := store.connectMongo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collection := database.Collection("test")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return &mongoCollection{store: store, collection: collection}
}

func (m *mongoCollection) insert(t *testing.T, docs ...bson.M) {
	t.Helper()
	for _, doc := range docs {
		if _, err := m.collection.InsertOne(context.Background(), doc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
