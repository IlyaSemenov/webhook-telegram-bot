package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sebdah/goldie/v2"
)

const chatID int64 = 123456789

// mockPipelineMessageSender captures sent messages for testing
type mockPipelineMessageSender struct {
	sentMessages     []sentMessage
	pipelineMessages []pipelineMessage
}

type sentMessage struct {
	chatID int64
	text   string
}

// pipelineMessage is a call that sends or updates a workflow run message.
type pipelineMessage struct {
	chatID      int64
	pipelineURL string
	text        string
	final       bool
	updateOnly  bool
}

func (m *mockPipelineMessageSender) SendMessage(chatID int64, text string) error {
	m.sentMessages = append(m.sentMessages, sentMessage{chatID: chatID, text: text})
	return nil
}

func (m *mockPipelineMessageSender) SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string, final bool) error {
	m.pipelineMessages = append(m.pipelineMessages, pipelineMessage{chatID: chatID, pipelineURL: pipelineURL, text: text, final: final})
	return nil
}

func (m *mockPipelineMessageSender) FinishPipelineMessage(chatID int64, pipelineURL string, text string) error {
	m.pipelineMessages = append(m.pipelineMessages, pipelineMessage{chatID: chatID, pipelineURL: pipelineURL, text: text, final: true, updateOnly: true})
	return nil
}

func newService() (*GitHubService, *mockPipelineMessageSender) {
	mock := &mockPipelineMessageSender{}
	return NewGitHubService(mock), mock
}

// fixture is the JSON format stored in testdata/*.json
type fixture struct {
	EventType      string                 `json:"event_type"`
	BranchFilters  []string               `json:"branch_filters,omitempty"`
	IncludeProject bool                   `json:"include_project,omitempty"`
	IsPipeline     bool                   `json:"is_pipeline,omitempty"`
	Final          bool                   `json:"final,omitempty"`
	UpdateOnly     bool                   `json:"update_only,omitempty"`
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
				if len(mock.sentMessages) != 0 || len(mock.pipelineMessages) != 0 {
					t.Errorf("expected no output, got sent=%d pipeline=%d",
						len(mock.sentMessages), len(mock.pipelineMessages))
				}
				return
			}

			var actual string
			if f.IsPipeline {
				if len(mock.sentMessages) != 0 || len(mock.pipelineMessages) != 1 {
					t.Fatalf("expected 1 pipeline message, got sent=%d pipeline=%d",
						len(mock.sentMessages), len(mock.pipelineMessages))
				}
				message := mock.pipelineMessages[0]
				if message.chatID != chatID {
					t.Errorf("chatID = %d, want %d", message.chatID, chatID)
				}
				if message.final != f.Final || message.updateOnly != f.UpdateOnly {
					t.Errorf("final = %v, updateOnly = %v, want %v, %v", message.final, message.updateOnly, f.Final, f.UpdateOnly)
				}
				actual = message.text
			} else {
				if len(mock.sentMessages) != 1 || len(mock.pipelineMessages) != 0 {
					t.Fatalf("expected 1 message, got sent=%d pipeline=%d",
						len(mock.sentMessages), len(mock.pipelineMessages))
				}
				if mock.sentMessages[0].chatID != chatID {
					t.Errorf("chatID = %d, want %d", mock.sentMessages[0].chatID, chatID)
				}
				actual = mock.sentMessages[0].text
			}

			goldie.New(t).Assert(t, name, []byte(actual))
		})
	}
}

func TestWorkflowRunMessageURL(t *testing.T) {
	// Events of one run attempt must pass the same URL so that the real implementation updates one Telegram message,
	// and a re-run must pass another URL so that it gets a new message.
	svc, mock := newService()

	handle := func(action string, attempt int) {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{
			"action": action,
			"workflow_run": map[string]any{
				"name":        "CI",
				"html_url":    "https://github.com/owner/repo/actions/runs/1",
				"run_attempt": attempt,
				"conclusion":  "success",
			},
		})
		if err := svc.HandleEvent(chatID, "workflow_run", payload, nil, false); err != nil {
			t.Fatalf("HandleEvent error: %v", err)
		}
	}

	handle("in_progress", 1)
	handle("completed", 1)
	handle("in_progress", 2)

	var urls []string
	for _, message := range mock.pipelineMessages {
		urls = append(urls, message.pipelineURL)
	}
	want := []string{
		"https://github.com/owner/repo/actions/runs/1/attempts/1",
		"https://github.com/owner/repo/actions/runs/1/attempts/1",
		"https://github.com/owner/repo/actions/runs/1/attempts/2",
	}
	if !slices.Equal(urls, want) {
		t.Errorf("pipeline URLs = %q, want %q", urls, want)
	}
}

func TestHandleEvent_UnsupportedEventType(t *testing.T) {
	svc, _ := newService()
	err := svc.HandleEvent(chatID, "unknown", nil, nil, false)
	if err == nil {
		t.Fatal("expected error for unsupported event type")
	}
}
