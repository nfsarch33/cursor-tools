package docsaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Component       string `json:"component"`
	HasDoc          bool   `json:"has_doc"`
	HasInstaller    bool   `json:"has_installer"`
	DocPath         string `json:"doc_path"`
	Freshness       string `json:"freshness,omitempty"`
	ComponentExists bool   `json:"component_exists"`
	Message         string `json:"message,omitempty"`
}

type Finding struct {
	Component string `json:"component"`
	HasDoc    bool   `json:"has_doc"`
	Message   string `json:"message"`
}

type Report struct {
	Results  []Result  `json:"results"`
	Findings []Finding `json:"findings"`
}

func (r Report) OK() bool {
	return len(r.Findings) == 0
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

var skillTriggerKeywords = []string{
	"use when",
	"trigger",
	"triggered when",
	"triggered by",
}

func Audit(root string, reg *Registry) Report {
	var report Report

	for _, comp := range reg.Components() {
		result := Result{
			Component:    comp.Name,
			HasInstaller: comp.HasInstaller,
			DocPath:      comp.DocPath,
		}

		compPath := filepath.Join(root, comp.Path)
		if _, err := os.Stat(compPath); err != nil {
			result.ComponentExists = false
			result.Message = fmt.Sprintf("component not found at %s", comp.Path)
			report.Results = append(report.Results, result)
			continue
		}
		result.ComponentExists = true

		docPath := filepath.Join(root, comp.DocPath)
		docInfo, err := os.Stat(docPath)
		if err != nil {
			result.HasDoc = false
			report.Findings = append(report.Findings, Finding{
				Component: comp.Name,
				HasDoc:    false,
				Message:   fmt.Sprintf("missing doc: %s", comp.DocPath),
			})
			report.Results = append(report.Results, result)
			continue
		}

		result.HasDoc = true
		result.Freshness = freshnessLabel(docInfo.ModTime())

		if comp.DocCheck == DocCheckSkillTrigger {
			raw, readErr := os.ReadFile(docPath)
			if readErr != nil {
				report.Findings = append(report.Findings, Finding{
					Component: comp.Name,
					HasDoc:    true,
					Message:   fmt.Sprintf("cannot read doc for trigger check: %v", readErr),
				})
				report.Results = append(report.Results, result)
				continue
			}
			lower := strings.ToLower(string(raw))
			hasTrigger := false
			for _, kw := range skillTriggerKeywords {
				if strings.Contains(lower, kw) {
					hasTrigger = true
					break
				}
			}
			if !hasTrigger {
				report.Findings = append(report.Findings, Finding{
					Component: comp.Name,
					HasDoc:    true,
					Message:   fmt.Sprintf("SKILL.md missing trigger conditions (expected keywords: %s)", strings.Join(skillTriggerKeywords, ", ")),
				})
			}
		}

		report.Results = append(report.Results, result)
	}

	return report
}

func freshnessLabel(modTime time.Time) string {
	age := time.Since(modTime)
	switch {
	case age < 7*24*time.Hour:
		return "fresh"
	case age < 30*24*time.Hour:
		return "recent"
	case age < 90*24*time.Hour:
		return "aging"
	default:
		return "stale"
	}
}

func AutoScanRules(root string) *Registry {
	reg := NewRegistry()
	rulesDir := filepath.Join(root, "cursor-config", "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return reg
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mdc") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".mdc")
		reg.Add(Component{
			Name:     name + "-rule",
			Kind:     KindRule,
			Path:     filepath.Join("cursor-config", "rules", e.Name()),
			DocPath:  filepath.Join("sop", name+".md"),
			DocCheck: DocCheckExists,
		})
	}
	return reg
}

func AutoScanSkills(root, skillsRelDir string) *Registry {
	reg := NewRegistry()
	skillsDir := filepath.Join(root, skillsRelDir)
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return reg
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsRelDir, e.Name(), "SKILL.md")
		if _, err := os.Stat(filepath.Join(root, skillFile)); err != nil {
			continue
		}
		reg.Add(Component{
			Name:     e.Name(),
			Kind:     KindSkill,
			Path:     skillFile,
			DocPath:  skillFile,
			DocCheck: DocCheckSkillTrigger,
		})
	}
	return reg
}

func AutoScanLaunchAgents(root string) *Registry {
	reg := NewRegistry()
	plistDir := filepath.Join(root, "LaunchAgents")
	entries, err := os.ReadDir(plistDir)
	if err != nil {
		return reg
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".plist")
		reg.Add(Component{
			Name:     name,
			Kind:     KindLaunchAgent,
			Path:     filepath.Join("LaunchAgents", e.Name()),
			DocPath:  filepath.Join("docs", "launchagent-"+name+".md"),
			DocCheck: DocCheckExists,
		})
	}
	return reg
}
