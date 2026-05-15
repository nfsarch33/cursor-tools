// runx-public-repo-gate: allow-file network_topology — fleet health default endpoints include loopback addresses for local-stack probes

package fleethealth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type NodeType string

const (
	NodeTypeWindows NodeType = "windows"
	NodeTypeWSL     NodeType = "wsl"
	NodeTypeMacOS   NodeType = "macos"
	NodeTypeOCI     NodeType = "oci"
)

type NodeState string

const (
	NodeStateAvailable       NodeState = "available"
	NodeStateOnboarding      NodeState = "onboarding"
	NodeStateActive          NodeState = "active"
	NodeStateQuarantine      NodeState = "quarantine"
	NodeStateDecommissioning NodeState = "decommissioning"
	NodeStateDecommissioned  NodeState = "decommissioned"
)

type ProbeStatus string

const (
	ProbePass    ProbeStatus = "pass"
	ProbeFail    ProbeStatus = "fail"
	ProbeSkipped ProbeStatus = "skipped"
	ProbeTimeout ProbeStatus = "timeout"
)

type Node struct {
	Name      string    `json:"name" yaml:"name"`
	Type      NodeType  `json:"type" yaml:"type"`
	SSHAlias  string    `json:"ssh_alias" yaml:"ssh_alias"`
	SSHPort   int       `json:"ssh_port" yaml:"ssh_port"`
	State     NodeState `json:"state" yaml:"state"`
	CanaryCmd string    `json:"canary_cmd" yaml:"canary_cmd"`
	HasDocker bool      `json:"has_docker" yaml:"has_docker"`
	HasMem0   bool      `json:"has_mem0" yaml:"has_mem0"`
}

type ProbeResult struct {
	Timestamp time.Time   `json:"ts"`
	Node      string      `json:"node"`
	Probe     string      `json:"probe"`
	Status    ProbeStatus `json:"status"`
	Latency   string      `json:"latency_ms,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type HealthReport struct {
	Timestamp time.Time     `json:"ts"`
	Node      string        `json:"node"`
	SSH       ProbeStatus   `json:"ssh"`
	Docker    ProbeStatus   `json:"docker"`
	Mem0      ProbeStatus   `json:"mem0"`
	Overall   ProbeStatus   `json:"overall"`
	Probes    []ProbeResult `json:"probes"`
}

type Config struct {
	Nodes         []Node        `json:"nodes" yaml:"nodes"`
	SSHTimeout    time.Duration `json:"ssh_timeout" yaml:"ssh_timeout"`
	ProbeInterval time.Duration `json:"probe_interval" yaml:"probe_interval"`
	OutputPath    string        `json:"output_path" yaml:"output_path"`
	Logger        *slog.Logger
}

func DefaultConfig() Config {
	return Config{
		SSHTimeout:    10 * time.Second,
		ProbeInterval: 5 * time.Minute,
		OutputPath:    os.ExpandEnv("$HOME/logs/runx/fleet-health.ndjson"),
		Logger:        slog.Default(),
	}
}

func ProbeSSH(ctx context.Context, node Node, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Timestamp: start,
		Node:      node.Name,
		Probe:     "ssh",
	}

	if node.State != NodeStateActive {
		result.Status = ProbeSkipped
		return result
	}

	canary := node.CanaryCmd
	if canary == "" {
		if node.Type == NodeTypeWindows {
			canary = "ssh-canary-win"
		} else {
			canary = "ssh-canary-wsl"
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "runx", "ssh", "exec",
		"--target", node.SSHAlias, "--cmd", canary)
	out, err := cmd.CombinedOutput()

	result.Latency = fmt.Sprintf("%d", time.Since(start).Milliseconds())

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Status = ProbeTimeout
			result.Error = "ssh timeout"
		} else {
			result.Status = ProbeFail
			result.Error = strings.TrimSpace(string(out))
			if len(result.Error) > 200 {
				result.Error = result.Error[:200]
			}
		}
		return result
	}

	result.Status = ProbePass
	return result
}

func ProbeDocker(ctx context.Context, node Node, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Timestamp: start,
		Node:      node.Name,
		Probe:     "docker",
	}

	if !node.HasDocker || node.State != NodeStateActive {
		result.Status = ProbeSkipped
		return result
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "runx", "ssh", "exec",
		"--target", node.SSHAlias,
		"--cmd", "docker compose ps --format json 2>/dev/null | head -20")
	out, err := cmd.CombinedOutput()

	result.Latency = fmt.Sprintf("%d", time.Since(start).Milliseconds())

	if err != nil {
		result.Status = ProbeFail
		result.Error = strings.TrimSpace(string(out))
		if len(result.Error) > 200 {
			result.Error = result.Error[:200]
		}
		return result
	}

	result.Status = ProbePass
	return result
}

func ProbeMem0(ctx context.Context, node Node, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Timestamp: start,
		Node:      node.Name,
		Probe:     "mem0",
	}

	if !node.HasMem0 || node.State != NodeStateActive {
		result.Status = ProbeSkipped
		return result
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "runx", "ssh", "exec",
		"--target", node.SSHAlias,
		"--cmd", "curl -sf http://127.0.0.1:18888/healthz")
	out, err := cmd.CombinedOutput()

	result.Latency = fmt.Sprintf("%d", time.Since(start).Milliseconds())

	if err != nil {
		result.Status = ProbeFail
		result.Error = strings.TrimSpace(string(out))
		if len(result.Error) > 200 {
			result.Error = result.Error[:200]
		}
		return result
	}

	result.Status = ProbePass
	return result
}

func ProbeNode(ctx context.Context, node Node, timeout time.Duration) HealthReport {
	report := HealthReport{
		Timestamp: time.Now(),
		Node:      node.Name,
	}

	ssh := ProbeSSH(ctx, node, timeout)
	docker := ProbeDocker(ctx, node, timeout)
	mem0 := ProbeMem0(ctx, node, timeout)

	report.SSH = ssh.Status
	report.Docker = docker.Status
	report.Mem0 = mem0.Status
	report.Probes = []ProbeResult{ssh, docker, mem0}

	report.Overall = ProbePass
	for _, p := range report.Probes {
		if p.Status == ProbeFail || p.Status == ProbeTimeout {
			report.Overall = ProbeFail
			break
		}
	}

	return report
}

func ProbeAll(ctx context.Context, cfg Config) []HealthReport {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		reports []HealthReport
	)

	for _, node := range cfg.Nodes {
		if node.State == NodeStateDecommissioned {
			continue
		}
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			r := ProbeNode(ctx, n, cfg.SSHTimeout)
			mu.Lock()
			reports = append(reports, r)
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	return reports
}

func WriteNDJSON(path string, reports []HealthReport) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open ndjson: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, r := range reports {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("encode report for %s: %w", r.Node, err)
		}
	}
	return nil
}

// Run starts the fleet-health daemon loop: probe all nodes at cfg.ProbeInterval,
// write NDJSON, repeat until ctx is cancelled.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.Logger.Info("fleet-health daemon starting",
		"nodes", len(cfg.Nodes),
		"interval", cfg.ProbeInterval.String(),
		"output", cfg.OutputPath,
	)

	tick := func() {
		reports := ProbeAll(ctx, cfg)
		if err := WriteNDJSON(cfg.OutputPath, reports); err != nil {
			cfg.Logger.Error("write ndjson failed", "error", err)
		}
		for _, r := range reports {
			cfg.Logger.Info("probe cycle",
				"node", r.Node, "ssh", r.SSH,
				"docker", r.Docker, "mem0", r.Mem0,
				"overall", r.Overall,
			)
		}
	}

	tick()

	ticker := time.NewTicker(cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cfg.Logger.Info("fleet-health daemon stopping")
			return ctx.Err()
		case <-ticker.C:
			tick()
		}
	}
}

// ReadLatest reads the NDJSON file at path and returns the most recent
// HealthReport per node. Used by the runx fleet-health subcommand.
func ReadLatest(path string) (map[string]HealthReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ndjson: %w", err)
	}

	latest := make(map[string]HealthReport)
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var r HealthReport
		if err := dec.Decode(&r); err != nil {
			continue
		}
		if prev, ok := latest[r.Node]; !ok || r.Timestamp.After(prev.Timestamp) {
			latest[r.Node] = r
		}
	}
	return latest, nil
}

// FormatTable formats the latest health reports as a human-readable table.
func FormatTable(reports map[string]HealthReport) string {
	if len(reports) == 0 {
		return "No fleet health data available."
	}

	var b strings.Builder
	header := fmt.Sprintf("%-15s %-8s %-8s %-8s %-8s %s", "NODE", "SSH", "DOCKER", "MEM0", "OVERALL", "LAST_CHECK")
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("-", len(header)) + "\n")

	for _, r := range reports {
		b.WriteString(fmt.Sprintf("%-15s %-8s %-8s %-8s %-8s %s\n",
			r.Node,
			statusIcon(r.SSH),
			statusIcon(r.Docker),
			statusIcon(r.Mem0),
			statusIcon(r.Overall),
			r.Timestamp.Format("2006-01-02T15:04:05"),
		))
	}
	return b.String()
}

func statusIcon(s ProbeStatus) string {
	switch s {
	case ProbePass:
		return "PASS"
	case ProbeFail:
		return "FAIL"
	case ProbeSkipped:
		return "SKIP"
	case ProbeTimeout:
		return "TMOUT"
	default:
		return "?"
	}
}
