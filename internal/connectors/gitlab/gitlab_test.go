package gitlab

import (
	"encoding/json"
	"os"
	"path/filepath"
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

type pipelineMessage struct {
	chatID      int64
	pipelineURL string
	text        string
}

func (m *mockPipelineMessageSender) SendMessage(chatID int64, text string) error {
	m.sentMessages = append(m.sentMessages, sentMessage{chatID: chatID, text: text})
	return nil
}

func (m *mockPipelineMessageSender) SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string) error {
	m.pipelineMessages = append(m.pipelineMessages, pipelineMessage{chatID: chatID, pipelineURL: pipelineURL, text: text})
	return nil
}

func newService() (*GitLabService, *mockPipelineMessageSender) {
	mock := &mockPipelineMessageSender{}
	return NewGitLabService(mock), mock
}

// fixture is the JSON format stored in testdata/*.json
type fixture struct {
	EventType      string                 `json:"event_type"`
	BranchFilters  []string               `json:"branch_filters,omitempty"`
	IncludeProject bool                   `json:"include_project,omitempty"`
	IsPipeline     bool                   `json:"is_pipeline,omitempty"`
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

func TestGitLabEvents(t *testing.T) {
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
				if len(mock.pipelineMessages) != 1 {
					t.Fatalf("expected 1 pipeline message, got %d", len(mock.pipelineMessages))
				}
				if mock.pipelineMessages[0].chatID != chatID {
					t.Errorf("chatID = %d, want %d", mock.pipelineMessages[0].chatID, chatID)
				}
				actual = mock.pipelineMessages[0].text
			} else {
				if len(mock.sentMessages) != 1 {
					t.Fatalf("expected 1 message, got %d", len(mock.sentMessages))
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

func TestPipelineUpdatesSameURL(t *testing.T) {
	// Verify that multiple pipeline events for the same pipeline
	// pass the same pipelineURL to SendOrUpdatePipelineMessage,
	// enabling the real implementation to update the same Telegram message.
	svc, mock := newService()

	basePayload := map[string]interface{}{
		"object_attributes": map[string]interface{}{
			"id":  42,
			"ref": "main",
			"url": "https://gitlab.com/owner/repo/-/pipelines/42",
		},
		"project": map[string]string{
			"name":                "repo",
			"path_with_namespace": "owner/repo",
		},
		"user":   map[string]string{"name": "John"},
		"builds": []map[string]interface{}{},
	}

	// First event: pipeline running
	runningPayload := make(map[string]interface{})
	for k, v := range basePayload {
		runningPayload[k] = v
	}
	runningPayload["object_attributes"] = map[string]interface{}{
		"id":     42,
		"ref":    "main",
		"status": "running",
		"url":    "https://gitlab.com/owner/repo/-/pipelines/42",
	}

	runningJSON, _ := json.Marshal(runningPayload)
	err := svc.HandleEvent(chatID, "Pipeline Hook", runningJSON, nil, false)
	if err != nil {
		t.Fatalf("first HandleEvent error: %v", err)
	}

	// Second event: pipeline success (same pipeline URL)
	successPayload := make(map[string]interface{})
	for k, v := range basePayload {
		successPayload[k] = v
	}
	successPayload["object_attributes"] = map[string]interface{}{
		"id":       42,
		"ref":      "main",
		"status":   "success",
		"url":      "https://gitlab.com/owner/repo/-/pipelines/42",
		"duration": 120,
	}

	successJSON, _ := json.Marshal(successPayload)
	err = svc.HandleEvent(chatID, "Pipeline Hook", successJSON, nil, false)
	if err != nil {
		t.Fatalf("second HandleEvent error: %v", err)
	}

	// Both calls should have the same pipelineURL
	if len(mock.pipelineMessages) != 2 {
		t.Fatalf("expected 2 pipeline messages, got %d", len(mock.pipelineMessages))
	}

	url1 := mock.pipelineMessages[0].pipelineURL
	url2 := mock.pipelineMessages[1].pipelineURL

	if url1 != url2 {
		t.Errorf("pipeline URLs don't match:\n  first:  %s\n  second: %s", url1, url2)
	}
	if url1 != "https://gitlab.com/owner/repo/-/pipelines/42" {
		t.Errorf("pipelineURL = %q, want pipeline URL from event", url1)
	}

	// Verify messages have different content (different statuses)
	if mock.pipelineMessages[0].text == mock.pipelineMessages[1].text {
		t.Error("both messages have identical text, expected different statuses")
	}
}

func TestHandleEvent_UnsupportedEventType(t *testing.T) {
	svc, _ := newService()
	err := svc.HandleEvent(chatID, "Unknown Hook", nil, nil, false)
	if err == nil {
		t.Fatal("expected error for unsupported event type")
	}
}
