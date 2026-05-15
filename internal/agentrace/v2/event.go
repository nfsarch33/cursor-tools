package agentrace

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const SchemaVersion = "2.0.0"

type EventType string

const (
	EventToolCall     EventType = "tool_call"
	EventToolResult   EventType = "tool_result"
	EventLLMRequest   EventType = "llm_request"
	EventLLMResponse  EventType = "llm_response"
	EventSessionStart EventType = "session_start"
	EventSessionEnd   EventType = "session_end"
	EventError        EventType = "error"
	EventMetric       EventType = "metric"
	EventPattern      EventType = "pattern"
	EventHealing      EventType = "healing"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Event struct {
	SchemaVersion string    `json:"schema_version"`
	Timestamp     time.Time `json:"ts"`
	Type          EventType `json:"type"`
	SessionID     string    `json:"session_id"`
	SprintID      string    `json:"sprint_id,omitempty"`
	NodeID        string    `json:"node_id,omitempty"`
	AgentID       string    `json:"agent_id,omitempty"`
	Severity      Severity  `json:"severity"`
	Payload       any       `json:"payload"`
	Tags          []string  `json:"tags,omitempty"`
	ParentEventID string    `json:"parent_event_id,omitempty"`
	DurationMs    int64     `json:"duration_ms,omitempty"`
}

type ToolCallPayload struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
	Result string         `json:"result,omitempty"`
	Status string         `json:"status"`
}

type LLMPayload struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	InputTokens     int    `json:"input_tokens"`
	OutputTokens    int    `json:"output_tokens"`
	LatencyMs       int64  `json:"latency_ms"`
	CacheHit        bool   `json:"cache_hit"`
	ReasoningTokens int    `json:"reasoning_tokens,omitempty"`
}

type MetricPayload struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type PatternPayload struct {
	PatternID   string  `json:"pattern_id"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	Occurrences int     `json:"occurrences"`
	Category    string  `json:"category"`
}

type HealingPayload struct {
	FailureType string `json:"failure_type"`
	Action      string `json:"action"`
	Success     bool   `json:"success"`
	DryRun      bool   `json:"dry_run"`
	Rollback    string `json:"rollback,omitempty"`
}

func NewEvent(eventType EventType, sessionID string, payload any) Event {
	return Event{
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now(),
		Type:          eventType,
		SessionID:     sessionID,
		Severity:      SeverityInfo,
		Payload:       payload,
	}
}

type Writer struct {
	path string
	file *os.File
	enc  *json.Encoder
}

func NewWriter(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("agentrace writer: %w", err)
	}
	return &Writer{
		path: path,
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

func (w *Writer) Write(event Event) error {
	return w.enc.Encode(event)
}

func (w *Writer) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

type Reader struct {
	path string
}

func NewReader(path string) *Reader {
	return &Reader{path: path}
}

func (r *Reader) ReadAll() ([]Event, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("agentrace reader: %w", err)
	}

	var events []Event
	dec := json.NewDecoder(
		&byteReader{data: data, pos: 0},
	)
	for dec.More() {
		var e Event
		if err := dec.Decode(&e); err != nil {
			return events, fmt.Errorf("decode event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Reader) FilterByType(eventType EventType) ([]Event, error) {
	all, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var filtered []Event
	for _, e := range all {
		if e.Type == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (r *Reader) FilterBySession(sessionID string) ([]Event, error) {
	all, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var filtered []Event
	for _, e := range all {
		if e.SessionID == sessionID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

type byteReader struct {
	data []byte
	pos  int
}

func (br *byteReader) Read(p []byte) (int, error) {
	if br.pos >= len(br.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, br.data[br.pos:])
	br.pos += n
	return n, nil
}
