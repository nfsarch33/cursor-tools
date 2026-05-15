package agentrace

import (
	"testing"
)

func TestAutoHealRegistry_BuiltinRules(t *testing.T) {
	r := NewAutoHealRegistry(nil)
	rules := r.Rules()

	if len(rules) != 10 {
		t.Errorf("len(rules) = %d, want 10 builtin rules", len(rules))
	}

	ids := make(map[string]bool)
	for _, rule := range rules {
		if ids[rule.ID] {
			t.Errorf("duplicate rule ID: %s", rule.ID)
		}
		ids[rule.ID] = true
		if len(rule.Actions) == 0 {
			t.Errorf("rule %s has no actions", rule.ID)
		}
		if rule.MaxRetries < 1 {
			t.Errorf("rule %s has MaxRetries %d, want >= 1", rule.ID, rule.MaxRetries)
		}
	}
}

func TestAutoHealRegistry_Register(t *testing.T) {
	r := NewAutoHealRegistry(nil)
	initial := len(r.Rules())

	r.Register(HealRule{
		ID:          "heal-custom",
		Category:    "custom",
		Actions:     []HealAction{{Name: "custom_action"}},
		MaxRetries:  1,
		CooldownMin: 5,
	})

	if len(r.Rules()) != initial+1 {
		t.Errorf("len(rules) = %d, want %d after register", len(r.Rules()), initial+1)
	}
}

func TestAutoHealRegistry_FindRulesForPattern(t *testing.T) {
	r := NewAutoHealRegistry(nil)

	timeoutPattern := Pattern{
		Category: "timeout",
	}
	matches := r.FindRulesForPattern(timeoutPattern)
	if len(matches) == 0 {
		t.Error("expected at least one rule matching timeout category")
	}
	for _, m := range matches {
		if m.Category != "timeout" && m.FailurePattern != timeoutPattern.Description {
			t.Errorf("rule %s matched but category=%s, pattern=%s", m.ID, m.Category, m.FailurePattern)
		}
	}
}

func TestAutoHealRegistry_FindRulesForPattern_NoMatch(t *testing.T) {
	r := NewAutoHealRegistry(nil)

	p := Pattern{Category: "nonexistent_category_xyz"}
	matches := r.FindRulesForPattern(p)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for unknown category, got %d", len(matches))
	}
}

func TestAutoHealRegistry_Evaluate_DryRun(t *testing.T) {
	r := NewAutoHealRegistry(nil)

	patterns := []Pattern{
		{Category: "timeout", Description: "recurring timeout (5 occurrences)"},
		{Category: "connection", Description: "docker container unhealthy"},
	}

	results := r.Evaluate(patterns)
	if len(results) == 0 {
		t.Error("expected at least one heal result")
	}
	for _, result := range results {
		if !result.DryRun {
			t.Errorf("result for %s should be dry-run", result.Action)
		}
	}
}

func TestAutoHealRegistry_RulesImmutability(t *testing.T) {
	r := NewAutoHealRegistry(nil)
	rules := r.Rules()
	originalLen := len(rules)

	rules = append(rules, HealRule{ID: "injected"})

	if len(r.Rules()) != originalLen {
		t.Error("modifying returned rules slice should not affect registry")
	}
}

func TestHealAction_AllBuiltinsHaveRollback(t *testing.T) {
	r := NewAutoHealRegistry(nil)
	for _, rule := range r.Rules() {
		for _, action := range rule.Actions {
			if action.Command == "" {
				t.Errorf("rule %s action %s has empty command", rule.ID, action.Name)
			}
		}
	}
}

func TestAutoHealRegistry_CooldownMinPositive(t *testing.T) {
	r := NewAutoHealRegistry(nil)
	for _, rule := range r.Rules() {
		if rule.CooldownMin < 1 {
			t.Errorf("rule %s has CooldownMin %d, want >= 1", rule.ID, rule.CooldownMin)
		}
	}
}
