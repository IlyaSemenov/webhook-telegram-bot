package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebdah/goldie/v2"
)

const chatID int64 = 123456789

// mockMessageSender captures sent messages for testing
type mockMessageSender struct {
	sentMessages []sentMessage
}

type sentMessage struct {
	chatID int64
	text   string
}

func (m *mockMessageSender) SendMessage(chatID int64, text string) error {
	m.sentMessages = append(m.sentMessages, sentMessage{chatID: chatID, text: text})
	return nil
}

func newService() (*GitHubService, *mockMessageSender) {
	mock := &mockMessageSender{}
	return NewGitHubService(mock), mock
}

// fixture is the JSON format stored in testdata/*.json
type fixture struct {
	EventType      string                 `json:"event_type"`
	BranchFilters  []string               `json:"branch_filters,omitempty"`
	IncludeProject bool                   `json:"include_project,omitempty"`
	ExpectNoOutput bool                   `json:"expect_no_output,omitempty"`
	Payload        map[string]interface{} `json:"payload"`
}

// loadFixture reads and parses a test fixture from testdata/<name>.json.
func loadFixture(t *testing.T, name string) *fixture {
	t.Helper()
	path := filepath.Join("testdata", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	var f fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", path, err)
	}
	return &f
}

// payloadJSON returns the payload as JSON bytes.
func (f *fixture) payloadJSON(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(f.Payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return b
}

func TestGitHubEvents(t *testing.T) {
	// Discover all .json fixture files in testdata/
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal("failed to read testdata directory:", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".json")]
		t.Run(name, func(t *testing.T) {
			f := loadFixture(t, name)
			svc, mock := newService()

			err := svc.HandleEvent(chatID, f.EventType, f.payloadJSON(t), f.BranchFilters, f.IncludeProject)
			if err != nil {
				t.Fatalf("HandleEvent error: %v", err)
			}

			if f.ExpectNoOutput {
				if len(mock.sentMessages) != 0 {
					t.Errorf("expected no output, got: %q", mock.sentMessages[0].text)
				}
				return
			}

			if len(mock.sentMessages) != 1 {
				t.Fatalf("expected 1 message, got %d", len(mock.sentMessages))
			}
			if mock.sentMessages[0].chatID != chatID {
				t.Errorf("chatID = %d, want %d", mock.sentMessages[0].chatID, chatID)
			}

			goldie.New(t).Assert(t, name, []byte(mock.sentMessages[0].text))
		})
	}
}

func TestHandleEvent_UnsupportedEventType(t *testing.T) {
	svc, _ := newService()
	err := svc.HandleEvent(chatID, "unknown", nil, nil, false)
	if err == nil {
		t.Fatal("expected error for unsupported event type")
	}
}
