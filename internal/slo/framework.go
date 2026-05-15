package slo

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

type SLOLevel string

const (
	SLOCritical SLOLevel = "critical"
	SLOHigh     SLOLevel = "high"
	SLOMedium   SLOLevel = "medium"
	SLOLow      SLOLevel = "low"
)

type SLOStatus string

const (
	SLOHealthy   SLOStatus = "healthy"
	SLOWarning   SLOStatus = "warning"
	SLOBurning   SLOStatus = "burning"
	SLOExhausted SLOStatus = "exhausted"
)

type SLO struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Level       SLOLevel `json:"level"`
	Target      float64  `json:"target"`
	Window      string   `json:"window"`
}

type SLI struct {
	SLOName   string    `json:"slo_name"`
	Timestamp time.Time `json:"ts"`
	Good      int64     `json:"good"`
	Total     int64     `json:"total"`
	Value     float64   `json:"value"`
}

type ErrorBudget struct {
	SLOName     string    `json:"slo_name"`
	Target      float64   `json:"target"`
	Current     float64   `json:"current"`
	Remaining   float64   `json:"remaining"`
	BurnRate    float64   `json:"burn_rate"`
	Status      SLOStatus `json:"status"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

type Framework struct {
	slos []SLO
}

func NewFramework() *Framework {
	return &Framework{
		slos: defaultSLOs(),
	}
}

func (f *Framework) AddSLO(slo SLO) {
	f.slos = append(f.slos, slo)
}

func (f *Framework) SLOs() []SLO {
	cp := make([]SLO, len(f.slos))
	copy(cp, f.slos)
	return cp
}

func (f *Framework) FindSLO(name string) (SLO, bool) {
	for _, s := range f.slos {
		if s.Name == name {
			return s, true
		}
	}
	return SLO{}, false
}

func (f *Framework) ComputeSLI(sloName string, good, total int64) SLI {
	value := 0.0
	if total > 0 {
		value = float64(good) / float64(total) * 100
	}
	return SLI{
		SLOName:   sloName,
		Timestamp: time.Now(),
		Good:      good,
		Total:     total,
		Value:     math.Round(value*100) / 100,
	}
}

func (f *Framework) ComputeErrorBudget(sloName string, slis []SLI) ErrorBudget {
	slo, ok := f.FindSLO(sloName)
	if !ok {
		return ErrorBudget{SLOName: sloName, Status: SLOExhausted}
	}

	var totalGood, totalAll int64
	for _, sli := range slis {
		if sli.SLOName == sloName {
			totalGood += sli.Good
			totalAll += sli.Total
		}
	}

	current := 0.0
	if totalAll > 0 {
		current = float64(totalGood) / float64(totalAll) * 100
	}

	remaining := current - slo.Target
	budgetTotal := 100 - slo.Target
	burnRate := 0.0
	if budgetTotal > 0 {
		burnRate = (budgetTotal - remaining) / budgetTotal
	}

	status := SLOHealthy
	switch {
	case remaining <= 0:
		status = SLOExhausted
	case burnRate > 0.8:
		status = SLOBurning
	case burnRate > 0.5:
		status = SLOWarning
	}

	return ErrorBudget{
		SLOName:     sloName,
		Target:      slo.Target,
		Current:     math.Round(current*100) / 100,
		Remaining:   math.Round(remaining*100) / 100,
		BurnRate:    math.Round(burnRate*1000) / 1000,
		Status:      status,
		EvaluatedAt: time.Now(),
	}
}

func (f *Framework) WriteReport(path string, budgets []ErrorBudget) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open slo report: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, b := range budgets {
		if err := enc.Encode(b); err != nil {
			return fmt.Errorf("encode budget: %w", err)
		}
	}
	return nil
}

func defaultSLOs() []SLO {
	return []SLO{
		{Name: "api_availability", Description: "API endpoint availability", Level: SLOCritical, Target: 99.9, Window: "30d"},
		{Name: "api_latency_p95", Description: "API p95 latency < 500ms", Level: SLOHigh, Target: 99.0, Window: "30d"},
		{Name: "mem0_write_success", Description: "Mem0 write success rate", Level: SLOHigh, Target: 99.5, Window: "7d"},
		{Name: "fleet_ssh_canary", Description: "Fleet SSH canary pass rate", Level: SLOMedium, Target: 99.0, Window: "7d"},
		{Name: "evospine_cycle_complete", Description: "EvoSpine ORHEP cycle completion", Level: SLOLow, Target: 95.0, Window: "30d"},
	}
}
