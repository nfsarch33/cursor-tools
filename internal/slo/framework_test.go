package slo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFramework_DefaultSLOs(t *testing.T) {
	f := NewFramework()
	slos := f.SLOs()
	if len(slos) != 5 {
		t.Errorf("len(slos) = %d, want 5 default SLOs", len(slos))
	}
}

func TestFramework_AddSLO(t *testing.T) {
	f := NewFramework()
	initial := len(f.SLOs())
	f.AddSLO(SLO{Name: "custom", Target: 99.9, Level: SLOCritical, Window: "7d"})
	if len(f.SLOs()) != initial+1 {
		t.Errorf("len = %d, want %d after add", len(f.SLOs()), initial+1)
	}
}

func TestFramework_FindSLO(t *testing.T) {
	f := NewFramework()
	slo, ok := f.FindSLO("api_availability")
	if !ok {
		t.Fatal("expected to find api_availability")
	}
	if slo.Target != 99.9 {
		t.Errorf("Target = %f, want 99.9", slo.Target)
	}
	if slo.Level != SLOCritical {
		t.Errorf("Level = %q, want critical", slo.Level)
	}
}

func TestFramework_FindSLO_NotFound(t *testing.T) {
	f := NewFramework()
	_, ok := f.FindSLO("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent SLO")
	}
}

func TestFramework_ComputeSLI(t *testing.T) {
	f := NewFramework()
	sli := f.ComputeSLI("api_availability", 999, 1000)
	if sli.Value != 99.9 {
		t.Errorf("Value = %f, want 99.9", sli.Value)
	}
	if sli.Good != 999 || sli.Total != 1000 {
		t.Errorf("Good=%d Total=%d, want 999/1000", sli.Good, sli.Total)
	}
}

func TestFramework_ComputeSLI_ZeroTotal(t *testing.T) {
	f := NewFramework()
	sli := f.ComputeSLI("api_availability", 0, 0)
	if sli.Value != 0 {
		t.Errorf("Value = %f, want 0 for zero total", sli.Value)
	}
}

func TestFramework_ComputeErrorBudget_Healthy(t *testing.T) {
	f := NewFramework()
	slis := []SLI{
		{SLOName: "api_availability", Good: 9999, Total: 10000},
	}
	budget := f.ComputeErrorBudget("api_availability", slis)
	if budget.Status != SLOHealthy {
		t.Errorf("Status = %q, want healthy (current 99.99 > target 99.9)", budget.Status)
	}
	if budget.Remaining <= 0 {
		t.Errorf("Remaining = %f, want > 0", budget.Remaining)
	}
}

func TestFramework_ComputeErrorBudget_Exhausted(t *testing.T) {
	f := NewFramework()
	slis := []SLI{
		{SLOName: "api_availability", Good: 9900, Total: 10000},
	}
	budget := f.ComputeErrorBudget("api_availability", slis)
	if budget.Status != SLOExhausted {
		t.Errorf("Status = %q, want exhausted (current 99.0 < target 99.9)", budget.Status)
	}
}

func TestFramework_ComputeErrorBudget_UnknownSLO(t *testing.T) {
	f := NewFramework()
	budget := f.ComputeErrorBudget("nonexistent", nil)
	if budget.Status != SLOExhausted {
		t.Errorf("Status = %q, want exhausted for unknown SLO", budget.Status)
	}
}

func TestFramework_WriteReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slo-report.ndjson")

	f := NewFramework()
	budgets := []ErrorBudget{
		{SLOName: "api_availability", Status: SLOHealthy, Current: 99.95, Target: 99.9},
	}

	if err := f.WriteReport(path, budgets); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded ErrorBudget
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Status != SLOHealthy {
		t.Errorf("Status = %q, want healthy", decoded.Status)
	}
}

func TestSLO_Immutability(t *testing.T) {
	f := NewFramework()
	slos := f.SLOs()
	slos[0].Name = "tampered"
	if f.SLOs()[0].Name == "tampered" {
		t.Error("modifying returned slice should not affect framework")
	}
}

func TestDefaultSLOs_AllHaveTargets(t *testing.T) {
	slos := defaultSLOs()
	for _, slo := range slos {
		if slo.Target <= 0 || slo.Target > 100 {
			t.Errorf("SLO %s has invalid target %f", slo.Name, slo.Target)
		}
		if slo.Window == "" {
			t.Errorf("SLO %s has empty window", slo.Name)
		}
	}
}
