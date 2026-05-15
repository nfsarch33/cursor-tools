package agentrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	e := NewEvent(EventToolCall, "sess-1", ToolCallPayload{
		Name:   "read_file",
		Status: "success",
	})

	if e.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", e.SchemaVersion, SchemaVersion)
	}
	if e.Type != EventToolCall {
		t.Errorf("Type = %q, want tool_call", e.Type)
	}
	if e.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", e.SessionID)
	}
	if e.Severity != SeverityInfo {
		t.Errorf("Severity = %q, want info", e.Severity)
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	e := Event{
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now().Truncate(time.Second),
		Type:          EventLLMRequest,
		SessionID:     "sess-42",
		SprintID:      "v424",
		Severity:      SeverityInfo,
		Payload: LLMPayload{
			Provider:     "anthropic",
			Model:        "claude-opus-4",
			InputTokens:  1000,
			OutputTokens: 500,
			LatencyMs:    2500,
		},
		Tags: []string{"phase3", "evospine"},
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.SessionID != e.SessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, e.SessionID)
	}
	if decoded.Type != e.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, e.Type)
	}
	if decoded.SprintID != e.SprintID {
		t.Errorf("SprintID = %q, want %q", decoded.SprintID, e.SprintID)
	}
}

func TestWriter_WriteSingle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	e := NewEvent(EventSessionStart, "sess-1", nil)
	if err := w.Write(e); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != EventSessionStart {
		t.Errorf("Type = %q, want session_start", decoded.Type)
	}
}

func TestWriter_WriteMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 5; i++ {
		e := NewEvent(EventMetric, "sess-1", MetricPayload{
			Name:  "token_count",
			Value: float64(i * 100),
			Unit:  "tokens",
		})
		if err := w.Write(e); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 5 {
		t.Errorf("lines = %d, want 5", lines)
	}
}

func TestReader_ReadAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	events := []Event{
		NewEvent(EventSessionStart, "sess-1", nil),
		NewEvent(EventToolCall, "sess-1", ToolCallPayload{Name: "read", Status: "ok"}),
		NewEvent(EventSessionEnd, "sess-1", nil),
	}
	for _, e := range events {
		if err := w.Write(e); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	w.Close()

	r := NewReader(path)
	read, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(read) != 3 {
		t.Fatalf("len = %d, want 3", len(read))
	}
	if read[0].Type != EventSessionStart {
		t.Errorf("first type = %q, want session_start", read[0].Type)
	}
	if read[2].Type != EventSessionEnd {
		t.Errorf("last type = %q, want session_end", read[2].Type)
	}
}

func TestReader_FilterByType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	w.Write(NewEvent(EventToolCall, "s1", ToolCallPayload{Name: "a", Status: "ok"}))
	w.Write(NewEvent(EventLLMRequest, "s1", LLMPayload{Model: "gpt-5"}))
	w.Write(NewEvent(EventToolCall, "s1", ToolCallPayload{Name: "b", Status: "ok"}))
	w.Close()

	r := NewReader(path)
	filtered, err := r.FilterByType(EventToolCall)
	if err != nil {
		t.Fatalf("FilterByType: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("len = %d, want 2 tool_call events", len(filtered))
	}
}

func TestReader_FilterBySession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.ndjson")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	w.Write(NewEvent(EventToolCall, "sess-A", nil))
	w.Write(NewEvent(EventToolCall, "sess-B", nil))
	w.Write(NewEvent(EventToolCall, "sess-A", nil))
	w.Close()

	r := NewReader(path)
	filtered, err := r.FilterBySession("sess-A")
	if err != nil {
		t.Fatalf("FilterBySession: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("len = %d, want 2 events for sess-A", len(filtered))
	}
}

func TestEventType_Constants(t *testing.T) {
	types := []EventType{
		EventToolCall, EventToolResult, EventLLMRequest, EventLLMResponse,
		EventSessionStart, EventSessionEnd, EventError, EventMetric,
		EventPattern, EventHealing,
	}
	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate event type: %v", et)
		}
		seen[et] = true
	}
	if len(types) != 10 {
		t.Errorf("expected 10 event types, got %d", len(types))
	}
}

func TestPatternPayload_JSON(t *testing.T) {
	p := PatternPayload{
		PatternID:   "pat-100",
		Description: "repeated ssh timeout",
		Confidence:  0.85,
		Occurrences: 12,
		Category:    "infrastructure",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded PatternPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.PatternID != p.PatternID {
		t.Errorf("PatternID = %q, want %q", decoded.PatternID, p.PatternID)
	}
	if decoded.Confidence != p.Confidence {
		t.Errorf("Confidence = %f, want %f", decoded.Confidence, p.Confidence)
	}
}

func TestHealingPayload_JSON(t *testing.T) {
	h := HealingPayload{
		FailureType: "mem0_timeout",
		Action:      "tunnel_restart",
		Success:     true,
		DryRun:      false,
		Rollback:    "kill tunnel PID",
	}

	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded HealingPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Action != h.Action {
		t.Errorf("Action = %q, want %q", decoded.Action, h.Action)
	}
}
