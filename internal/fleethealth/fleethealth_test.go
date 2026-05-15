package fleethealth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SSHTimeout != 10*time.Second {
		t.Errorf("SSHTimeout = %v, want 10s", cfg.SSHTimeout)
	}
	if cfg.ProbeInterval != 5*time.Minute {
		t.Errorf("ProbeInterval = %v, want 5m", cfg.ProbeInterval)
	}
	if cfg.OutputPath == "" {
		t.Error("OutputPath should not be empty")
	}
}

func TestProbeSSH_SkipsNonActive(t *testing.T) {
	node := Node{
		Name:     "test-node",
		Type:     NodeTypeWSL,
		SSHAlias: "test",
		State:    NodeStateDecommissioned,
	}
	result := ProbeSSH(context.Background(), node, 5*time.Second)
	if result.Status != ProbeSkipped {
		t.Errorf("Status = %v, want skipped for decommissioned node", result.Status)
	}
}

func TestProbeDocker_SkipsNoDocker(t *testing.T) {
	node := Node{
		Name:      "test-node",
		Type:      NodeTypeWSL,
		SSHAlias:  "test",
		State:     NodeStateActive,
		HasDocker: false,
	}
	result := ProbeDocker(context.Background(), node, 5*time.Second)
	if result.Status != ProbeSkipped {
		t.Errorf("Status = %v, want skipped for non-docker node", result.Status)
	}
}

func TestProbeMem0_SkipsNoMem0(t *testing.T) {
	node := Node{
		Name:     "test-node",
		Type:     NodeTypeWSL,
		SSHAlias: "test",
		State:    NodeStateActive,
		HasMem0:  false,
	}
	result := ProbeMem0(context.Background(), node, 5*time.Second)
	if result.Status != ProbeSkipped {
		t.Errorf("Status = %v, want skipped for non-mem0 node", result.Status)
	}
}

func TestProbeNode_AllSkipped(t *testing.T) {
	node := Node{
		Name:     "offline-node",
		Type:     NodeTypeWSL,
		SSHAlias: "test",
		State:    NodeStateAvailable,
	}
	report := ProbeNode(context.Background(), node, 5*time.Second)
	if report.Node != "offline-node" {
		t.Errorf("Node = %v, want offline-node", report.Node)
	}
	if report.SSH != ProbeSkipped {
		t.Errorf("SSH = %v, want skipped", report.SSH)
	}
}

func TestProbeAll_SkipsDecommissioned(t *testing.T) {
	cfg := Config{
		SSHTimeout: 1 * time.Second,
		Nodes: []Node{
			{Name: "active", State: NodeStateActive, SSHAlias: "nonexistent-test"},
			{Name: "decom", State: NodeStateDecommissioned, SSHAlias: "test"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	reports := ProbeAll(ctx, cfg)
	for _, r := range reports {
		if r.Node == "decom" {
			t.Error("decommissioned node should not be probed")
		}
	}
	if len(reports) != 1 {
		t.Errorf("len(reports) = %d, want 1 (only active node)", len(reports))
	}
}

func TestWriteNDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-health.ndjson")

	reports := []HealthReport{
		{
			Timestamp: time.Now(),
			Node:      "test-node",
			SSH:       ProbePass,
			Docker:    ProbeSkipped,
			Mem0:      ProbeSkipped,
			Overall:   ProbePass,
			Probes:    []ProbeResult{},
		},
	}

	if err := WriteNDJSON(path, reports); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded HealthReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Node != "test-node" {
		t.Errorf("Node = %v, want test-node", decoded.Node)
	}
	if decoded.Overall != ProbePass {
		t.Errorf("Overall = %v, want pass", decoded.Overall)
	}
}

func TestWriteNDJSON_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-append.ndjson")

	r1 := []HealthReport{{Node: "node-1", Overall: ProbePass}}
	r2 := []HealthReport{{Node: "node-2", Overall: ProbeFail}}

	if err := WriteNDJSON(path, r1); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteNDJSON(path, r2); err != nil {
		t.Fatalf("second write: %v", err)
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
	if lines != 2 {
		t.Errorf("lines = %d, want 2 (one per report)", lines)
	}
}

func TestNodeState_Constants(t *testing.T) {
	states := []NodeState{
		NodeStateAvailable, NodeStateOnboarding, NodeStateActive,
		NodeStateQuarantine, NodeStateDecommissioning, NodeStateDecommissioned,
	}
	seen := make(map[NodeState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("duplicate state: %v", s)
		}
		seen[s] = true
		if s == "" {
			t.Error("empty state constant")
		}
	}
}

func TestRun_StopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "daemon-test.ndjson")

	cfg := Config{
		SSHTimeout:    1 * time.Second,
		ProbeInterval: 50 * time.Millisecond,
		OutputPath:    outPath,
		Nodes: []Node{
			{Name: "stub", Type: NodeTypeWSL, SSHAlias: "test", State: NodeStateAvailable},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := Run(ctx, cfg)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("output file not created: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("output file is empty; daemon should have written at least one cycle")
	}
}

func TestReadLatest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "read-test.ndjson")

	now := time.Now().Truncate(time.Second)
	old := now.Add(-1 * time.Hour)

	reports := []HealthReport{
		{Timestamp: old, Node: "node-a", SSH: ProbePass, Overall: ProbePass},
		{Timestamp: now, Node: "node-a", SSH: ProbeFail, Overall: ProbeFail},
		{Timestamp: now, Node: "node-b", SSH: ProbePass, Overall: ProbePass},
	}
	if err := WriteNDJSON(path, reports); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	latest, err := ReadLatest(path)
	if err != nil {
		t.Fatalf("ReadLatest: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("len(latest) = %d, want 2", len(latest))
	}
	if latest["node-a"].SSH != ProbeFail {
		t.Errorf("node-a SSH = %v, want fail (should pick latest)", latest["node-a"].SSH)
	}
	if latest["node-b"].SSH != ProbePass {
		t.Errorf("node-b SSH = %v, want pass", latest["node-b"].SSH)
	}
}

func TestReadLatest_FileNotFound(t *testing.T) {
	_, err := ReadLatest("/nonexistent/path/test.ndjson")
	if err == nil {
		t.Error("ReadLatest should fail for nonexistent file")
	}
}

func TestFormatTable(t *testing.T) {
	reports := map[string]HealthReport{
		"node-1": {Node: "node-1", SSH: ProbePass, Docker: ProbeSkipped, Mem0: ProbeSkipped, Overall: ProbePass, Timestamp: time.Now()},
	}
	table := FormatTable(reports)
	if !strings.Contains(table, "node-1") {
		t.Error("table should contain node name")
	}
	if !strings.Contains(table, "PASS") {
		t.Error("table should contain PASS status")
	}
}

func TestFormatTable_Empty(t *testing.T) {
	table := FormatTable(nil)
	if !strings.Contains(table, "No fleet health data") {
		t.Errorf("empty table = %q, want 'No fleet health data' message", table)
	}
}

func TestStatusIcon(t *testing.T) {
	cases := []struct {
		in   ProbeStatus
		want string
	}{
		{ProbePass, "PASS"},
		{ProbeFail, "FAIL"},
		{ProbeSkipped, "SKIP"},
		{ProbeTimeout, "TMOUT"},
		{ProbeStatus("unknown"), "?"},
	}
	for _, tc := range cases {
		got := statusIcon(tc.in)
		if got != tc.want {
			t.Errorf("statusIcon(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProbeResult_JSONRoundTrip(t *testing.T) {
	pr := ProbeResult{
		Timestamp: time.Now().Truncate(time.Second),
		Node:      "test",
		Probe:     "ssh",
		Status:    ProbePass,
		Latency:   "42",
	}

	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ProbeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Node != pr.Node || decoded.Probe != pr.Probe || decoded.Status != pr.Status {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, pr)
	}
}
