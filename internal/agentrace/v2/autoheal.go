// runx-public-repo-gate: allow-file secret_cred_ref — autoheal remediation templates reference SSH alias paths as example patterns, not live credentials

package agentrace

import (
	"fmt"
	"log/slog"
	"time"
)

type HealAction struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	DryRun   bool   `json:"dry_run"`
	Rollback string `json:"rollback,omitempty"`
}

type HealRule struct {
	ID             string       `json:"id"`
	FailurePattern string       `json:"failure_pattern"`
	Category       string       `json:"category"`
	Actions        []HealAction `json:"actions"`
	MaxRetries     int          `json:"max_retries"`
	CooldownMin    int          `json:"cooldown_min"`
}

type HealResult struct {
	Rule      string    `json:"rule"`
	Action    string    `json:"action"`
	Success   bool      `json:"success"`
	DryRun    bool      `json:"dry_run"`
	Timestamp time.Time `json:"ts"`
	Error     string    `json:"error,omitempty"`
}

type AutoHealRegistry struct {
	rules  []HealRule
	logger *slog.Logger
}

func NewAutoHealRegistry(logger *slog.Logger) *AutoHealRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &AutoHealRegistry{
		rules:  builtinRules(),
		logger: logger,
	}
}

func (r *AutoHealRegistry) Register(rule HealRule) {
	r.rules = append(r.rules, rule)
}

func (r *AutoHealRegistry) Rules() []HealRule {
	cp := make([]HealRule, len(r.rules))
	copy(cp, r.rules)
	return cp
}

func (r *AutoHealRegistry) FindRulesForPattern(pattern Pattern) []HealRule {
	var matches []HealRule
	for _, rule := range r.rules {
		if rule.Category == pattern.Category || rule.FailurePattern == pattern.Description {
			matches = append(matches, rule)
		}
	}
	return matches
}

func (r *AutoHealRegistry) Evaluate(patterns []Pattern) []HealResult {
	var results []HealResult
	for _, p := range patterns {
		rules := r.FindRulesForPattern(p)
		for _, rule := range rules {
			for _, action := range rule.Actions {
				results = append(results, HealResult{
					Rule:      rule.ID,
					Action:    action.Name,
					Success:   false,
					DryRun:    true,
					Timestamp: time.Now(),
				})
			}
		}
	}
	return results
}

func builtinRules() []HealRule {
	return []HealRule{
		{
			ID:             "heal-tunnel-restart",
			FailurePattern: "recurring timeout",
			Category:       "timeout",
			Actions: []HealAction{
				{Name: "restart_tunnel", Command: "runx tunnel daemon mem0-oracle", DryRun: true, Rollback: "kill tunnel PID"},
			},
			MaxRetries:  3,
			CooldownMin: 5,
		},
		{
			ID:             "heal-docker-restart",
			FailurePattern: "docker container unhealthy",
			Category:       "connection",
			Actions: []HealAction{
				{Name: "restart_container", Command: "docker compose restart <service>", DryRun: true, Rollback: "docker compose stop <service>"},
			},
			MaxRetries:  2,
			CooldownMin: 10,
		},
		{
			ID:             "heal-mem0-reconnect",
			FailurePattern: "mem0 api unreachable",
			Category:       "connection",
			Actions: []HealAction{
				{Name: "check_tunnel", Command: "runx nc mem0-oracle", DryRun: true},
				{Name: "restart_mem0_api", Command: "docker compose restart mem0-api", DryRun: true, Rollback: "docker compose stop mem0-api"},
			},
			MaxRetries:  3,
			CooldownMin: 5,
		},
		{
			ID:             "heal-stale-mcp",
			FailurePattern: "stale mcp process",
			Category:       "memory",
			Actions: []HealAction{
				{Name: "kill_stale_process", Command: "kill -TERM <pid>", DryRun: true, Rollback: "process already terminated"},
			},
			MaxRetries:  1,
			CooldownMin: 1,
		},
		{
			ID:             "heal-ssh-key-expired",
			FailurePattern: "ssh authentication failure",
			Category:       "authentication",
			Actions: []HealAction{
				{Name: "verify_key", Command: "ssh-add -l", DryRun: true},
				{Name: "add_key", Command: "ssh-add ~/.ssh/fleet_lan", DryRun: true},
			},
			MaxRetries:  2,
			CooldownMin: 15,
		},
		{
			ID:             "heal-oom-prevention",
			FailurePattern: "memory pressure critical",
			Category:       "memory",
			Actions: []HealAction{
				{Name: "kill_heaviest", Command: "kill -TERM $(ps aux --sort=-%mem | head -2 | tail -1 | awk '{print $2}')", DryRun: true},
			},
			MaxRetries:  1,
			CooldownMin: 30,
		},
		{
			ID:             "heal-git-lock",
			FailurePattern: "git index.lock stale",
			Category:       "general",
			Actions: []HealAction{
				{Name: "remove_lock", Command: "rm -f .git/index.lock", DryRun: true},
			},
			MaxRetries:  1,
			CooldownMin: 1,
		},
		{
			ID:             "heal-sentrux-baseline",
			FailurePattern: "sentrux baseline stale",
			Category:       "general",
			Actions: []HealAction{
				{Name: "refresh_baseline", Command: fmt.Sprintf("runx sentrux save --repo <alias>"), DryRun: true},
			},
			MaxRetries:  1,
			CooldownMin: 60,
		},
		{
			ID:             "heal-workspace-dirty",
			FailurePattern: "workspace doctor RED",
			Category:       "general",
			Actions: []HealAction{
				{Name: "run_cleanup", Command: "runx worktree cleanup --repo <alias>", DryRun: true},
			},
			MaxRetries:  1,
			CooldownMin: 30,
		},
		{
			ID:             "heal-permission-denied",
			FailurePattern: "ssh permission denied",
			Category:       "permission",
			Actions: []HealAction{
				{Name: "check_authorized_keys", Command: "ssh <node> cat ~/.ssh/authorized_keys | grep fleet_lan", DryRun: true},
			},
			MaxRetries:  1,
			CooldownMin: 60,
		},
	}
}
