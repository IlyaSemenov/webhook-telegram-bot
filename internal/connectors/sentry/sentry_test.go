package sentry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sebdah/goldie/v2"
)

const chatID int64 = 123456789

// mockMessageSender captures sent messages for testing and fails with err if it is set.
type mockMessageSender struct {
	sentMessages []sentMessage
	err          error
}

type sentMessage struct {
	chatID int64
	text   string
}

func (m *mockMessageSender) SendMessage(chatID int64, text string) error {
	if m.err != nil {
		return m.err
	}
	m.sentMessages = append(m.sentMessages, sentMessage{chatID: chatID, text: text})
	return nil
}

func newService() (*SentryService, *mockMessageSender) {
	mock := &mockMessageSender{}
	return NewSentryService(mock), mock
}

// fixture is the JSON format stored in testdata/*.json
type fixture struct {
	Resource       string                 `json:"resource"`
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

func (f *fixture) payloadJSON(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(f.Payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return b
}

func TestSentryEvents(t *testing.T) {
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

			err := svc.HandleEvent(chatID, f.Resource, f.payloadJSON(t), f.IncludeProject)
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

func TestHandleEvent_RejectsPayloadWithoutTitle(t *testing.T) {
	for _, resource := range []string{"event_alert", "metric_alert"} {
		t.Run(resource, func(t *testing.T) {
			svc, mock := newService()
			if err := svc.HandleEvent(chatID, resource, []byte(`{}`), false); err == nil {
				t.Fatal("expected error for payload without title")
			}
			if len(mock.sentMessages) != 0 {
				t.Errorf("messages = %+v, want none", mock.sentMessages)
			}
		})
	}

	t.Run("issue", func(t *testing.T) {
		svc, mock := newService()
		if err := svc.HandleEvent(chatID, "issue", []byte(`{"action":"created"}`), false); err == nil {
			t.Fatal("expected error for payload without title")
		}
		if len(mock.sentMessages) != 0 {
			t.Errorf("messages = %+v, want none", mock.sentMessages)
		}
	})
}

func TestFormatText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxLength int
		want      string
	}{
		{name: "escapes HTML", text: `<b>"x" & y</b>`, maxLength: 100, want: "&lt;b&gt;&#34;x&#34; &amp; y&lt;/b&gt;"},
		{name: "joins lines", text: " first\n\tsecond  third ", maxLength: 100, want: "first second third"},
		{name: "keeps text of the maximum length", text: "абвгд", maxLength: 5, want: "абвгд"},
		{name: "shortens by characters", text: "абвгдеж", maxLength: 5, want: "абвгд…"},
		{name: "does not cut an escape sequence", text: "abcd&efgh", maxLength: 5, want: "abcd&amp;…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatText(tt.text, tt.maxLength); got != tt.want {
				t.Errorf("formatText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessageFitsTelegramLimit(t *testing.T) {
	long := strings.Repeat("<&", 5000)
	payload, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"event": map[string]any{
				"title":       long,
				"culprit":     long,
				"environment": long,
				"url":         "https://sentry.example.com/api/0/projects/acme/" + strings.Repeat("p", 5000) + "/events/1/",
				"web_url":     "https://sentry.example.com/organizations/acme/issues/1/",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc, mock := newService()
	if err := svc.HandleEvent(chatID, "event_alert", payload, true); err != nil {
		t.Fatalf("HandleEvent error: %v", err)
	}

	unescaped := strings.NewReplacer("&lt;", "<", "&amp;", "&").Replace(mock.sentMessages[0].text)
	if length := utf8.RuneCountInString(unescaped); length > 4096 {
		t.Errorf("message length = %d, want at most 4096", length)
	}
}
