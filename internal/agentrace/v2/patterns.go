package agentrace

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Pattern struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Confidence  float64     `json:"confidence"`
	Occurrences int         `json:"occurrences"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	EventTypes  []EventType `json:"event_types"`
	Tags        []string    `json:"tags,omitempty"`
}

type PatternExtractor struct {
	minOccurrences int
	minConfidence  float64
}

func NewPatternExtractor(minOccurrences int, minConfidence float64) *PatternExtractor {
	return &PatternExtractor{
		minOccurrences: minOccurrences,
		minConfidence:  minConfidence,
	}
}

func (pe *PatternExtractor) ExtractFailurePatterns(events []Event) []Pattern {
	errorCounts := make(map[string]*errorStat)

	for _, e := range events {
		if e.Type != EventError && e.Severity != SeverityError && e.Severity != SeverityCritical {
			continue
		}

		key := fmt.Sprintf("%s:%s", e.Type, categorizeError(e))
		stat, ok := errorCounts[key]
		if !ok {
			stat = &errorStat{
				firstSeen: e.Timestamp,
				category:  categorizeError(e),
			}
			errorCounts[key] = stat
		}
		stat.count++
		stat.lastSeen = e.Timestamp
	}

	var patterns []Pattern
	for _, stat := range errorCounts {
		if stat.count < pe.minOccurrences {
			continue
		}
		confidence := float64(stat.count) / float64(len(events))
		if confidence < pe.minConfidence {
			confidence = pe.minConfidence
		}
		if confidence > 1.0 {
			confidence = 1.0
		}

		patterns = append(patterns, Pattern{
			ID:          fmt.Sprintf("pat-%s-%d", stat.category, stat.count),
			Description: fmt.Sprintf("recurring %s (%d occurrences)", stat.category, stat.count),
			Category:    stat.category,
			Confidence:  confidence,
			Occurrences: stat.count,
			FirstSeen:   stat.firstSeen,
			LastSeen:    stat.lastSeen,
			EventTypes:  []EventType{EventError},
		})
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Occurrences > patterns[j].Occurrences
	})

	return patterns
}

func (pe *PatternExtractor) ExtractSuccessPatterns(events []Event) []Pattern {
	toolSuccess := make(map[string]*successStat)

	for _, e := range events {
		if e.Type != EventToolCall {
			continue
		}
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			continue
		}
		name, _ := payload["name"].(string)
		status, _ := payload["status"].(string)
		if name == "" {
			continue
		}

		stat, ok := toolSuccess[name]
		if !ok {
			stat = &successStat{}
			toolSuccess[name] = stat
		}
		stat.total++
		if status == "success" || status == "ok" {
			stat.successes++
		}
	}

	var patterns []Pattern
	for name, stat := range toolSuccess {
		if stat.total < pe.minOccurrences {
			continue
		}
		rate := float64(stat.successes) / float64(stat.total)
		if rate >= 0.9 {
			patterns = append(patterns, Pattern{
				ID:          fmt.Sprintf("pat-success-%s", name),
				Description: fmt.Sprintf("reliable tool: %s (%.0f%% success, %d calls)", name, rate*100, stat.total),
				Category:    "tool_reliability",
				Confidence:  rate,
				Occurrences: stat.total,
				EventTypes:  []EventType{EventToolCall},
			})
		}
	}

	return patterns
}

type errorStat struct {
	count     int
	firstSeen time.Time
	lastSeen  time.Time
	category  string
}

type successStat struct {
	total     int
	successes int
}

func categorizeError(e Event) string {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return "unknown"
	}

	if errMsg, ok := payload["error"].(string); ok {
		lower := strings.ToLower(errMsg)
		switch {
		case strings.Contains(lower, "timeout"):
			return "timeout"
		case strings.Contains(lower, "connection"):
			return "connection"
		case strings.Contains(lower, "auth"):
			return "authentication"
		case strings.Contains(lower, "permission"):
			return "permission"
		case strings.Contains(lower, "not found"):
			return "not_found"
		case strings.Contains(lower, "oom") || strings.Contains(lower, "memory"):
			return "memory"
		default:
			return "general"
		}
	}
	return "unknown"
}
