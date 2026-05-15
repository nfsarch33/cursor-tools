package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Manage session handoff documents for crash safety",
	Long:  "Create and finalize handoff documents at TODO start/complete boundaries.",
}

var handoffInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a handoff document for the current TODO",
	RunE:  runHandoffInit,
}

var handoffFinalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Finalize a handoff document with completion evidence",
	RunE:  runHandoffFinalize,
}

var (
	handoffTodoID   string
	handoffRepo     string
	handoffBranch   string
	handoffContent  string
	handoffEvidence string
)

func init() {
	handoffCmd.AddCommand(handoffInitCmd)
	handoffCmd.AddCommand(handoffFinalizeCmd)

	handoffInitCmd.Flags().StringVar(&handoffTodoID, "todo-id", "", "TODO identifier")
	handoffInitCmd.Flags().StringVar(&handoffRepo, "repo", "", "Repository alias or path")
	handoffInitCmd.Flags().StringVar(&handoffBranch, "branch", "", "Current branch name")
	handoffInitCmd.Flags().StringVar(&handoffContent, "content", "", "TODO description")

	handoffFinalizeCmd.Flags().StringVar(&handoffTodoID, "todo-id", "", "TODO identifier")
	handoffFinalizeCmd.Flags().StringVar(&handoffEvidence, "evidence", "", "Completion evidence summary")

	rootCmd.AddCommand(handoffCmd)
}

func handoffDir() string {
	p := config.DefaultPaths()
	return filepath.Join(p.GlobalKB, "session-handoffs")
}

func handoffFilename(todoID string) string {
	date := time.Now().Format("2006-01-02")
	safe := strings.ReplaceAll(todoID, "/", "-")
	safe = strings.ReplaceAll(safe, " ", "-")
	return fmt.Sprintf("%s-%s-handoff.md", date, safe)
}

func runHandoffInit(_ *cobra.Command, _ []string) error {
	if handoffTodoID == "" {
		return fmt.Errorf("--todo-id is required")
	}

	dir := handoffDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create handoff dir: %w", err)
	}

	filename := handoffFilename(handoffTodoID)
	path := filepath.Join(dir, filename)

	now := time.Now().Format("2006-01-02T15:04:05-07:00")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Handoff: %s\n\n", handoffTodoID))
	sb.WriteString(fmt.Sprintf("**Started**: %s\n", now))
	sb.WriteString(fmt.Sprintf("**TODO**: %s\n", handoffContent))
	sb.WriteString(fmt.Sprintf("**Repo**: %s\n", handoffRepo))
	sb.WriteString(fmt.Sprintf("**Branch**: %s\n", handoffBranch))
	sb.WriteString("\n## Status\n\nIN PROGRESS\n")
	sb.WriteString("\n## Files Changed\n\n(auto-populated on finalize)\n")
	sb.WriteString("\n## Resume Instructions\n\n(auto-populated on finalize)\n")

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write handoff: %w", err)
	}

	fmt.Printf("Handoff created: %s\n", path)
	return nil
}

func runHandoffFinalize(_ *cobra.Command, _ []string) error {
	if handoffTodoID == "" {
		return fmt.Errorf("--todo-id is required")
	}

	dir := handoffDir()
	filename := handoffFilename(handoffTodoID)
	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read handoff: %w", err)
	}

	now := time.Now().Format("2006-01-02T15:04:05-07:00")
	content := string(data)
	content = strings.Replace(content, "## Status\n\nIN PROGRESS", fmt.Sprintf("## Status\n\nCOMPLETED at %s", now), 1)

	if handoffEvidence != "" {
		content = strings.Replace(content,
			"## Resume Instructions\n\n(auto-populated on finalize)",
			fmt.Sprintf("## Evidence\n\n%s\n\n## Resume Instructions\n\nN/A — completed", handoffEvidence),
			1)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write finalized handoff: %w", err)
	}

	fmt.Printf("Handoff finalized: %s\n", path)
	return nil
}
