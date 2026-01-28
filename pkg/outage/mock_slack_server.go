package outage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/slack-go/slack"
)

// PostedMessage represents a message posted to the mock Slack server.
type PostedMessage struct {
	Channel         string
	Text            string
	ThreadTimestamp string
	ResponseTS      string
}

// AddedReaction represents a reaction added to a message in the mock Slack server.
type AddedReaction struct {
	Channel   string
	Timestamp string
	Name      string
}

// MockSlackServer is a mock Slack API server for testing.
type MockSlackServer struct {
	server         *httptest.Server
	client         *slack.Client
	postedMsgs     []PostedMessage
	addedReactions []AddedReaction
	mu             sync.Mutex
	tsCounter      int64
	baseTS         int64
}

// NewMockSlackServer creates a new mock Slack API server for testing.
// It returns the server instance and a Slack client configured to use it.
func NewMockSlackServer(t *testing.T) *MockSlackServer {
	m := &MockSlackServer{
		postedMsgs:     make([]PostedMessage, 0),
		addedReactions: make([]AddedReaction, 0),
		baseTS:         1234567890,
		tsCounter:      0,
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		switch r.URL.Path {
		case "/api/chat.postMessage":
			// Capture the posted message
			m.tsCounter++
			// Generate sequential timestamp: baseTS.000001, baseTS.000002, etc.
			responseTS := fmt.Sprintf("%d.%06d", m.baseTS, m.tsCounter)
			msg := PostedMessage{
				Channel:         r.FormValue("channel"),
				Text:            r.FormValue("text"),
				ThreadTimestamp: r.FormValue("thread_ts"),
				ResponseTS:      responseTS,
			}
			m.postedMsgs = append(m.postedMsgs, msg)

			// Return successful Slack API response
			response := map[string]interface{}{
				"ok":      true,
				"channel": msg.Channel,
				"ts":      responseTS,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case "/api/reactions.add":
			// Capture the added reaction
			reaction := AddedReaction{
				Channel:   r.FormValue("channel"),
				Timestamp: r.FormValue("timestamp"),
				Name:      r.FormValue("name"),
			}
			m.addedReactions = append(m.addedReactions, reaction)

			// Return successful Slack API response
			response := map[string]interface{}{
				"ok": true,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		default:
			t.Errorf("Unexpected API path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	m.client = slack.New("test-token", slack.OptionAPIURL(m.server.URL+"/api/"))
	return m
}

// Client returns the Slack client configured to use this mock server.
func (m *MockSlackServer) Client() *slack.Client {
	return m.client
}

// PostedMessages returns all messages that were posted to the mock server.
func (m *MockSlackServer) PostedMessages() []PostedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PostedMessage, len(m.postedMsgs))
	copy(result, m.postedMsgs)
	return result
}

// AddedReactions returns all reactions that were added to messages in the mock server.
func (m *MockSlackServer) AddedReactions() []AddedReaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]AddedReaction, len(m.addedReactions))
	copy(result, m.addedReactions)
	return result
}

// Close shuts down the mock server.
func (m *MockSlackServer) Close() {
	m.server.Close()
}
