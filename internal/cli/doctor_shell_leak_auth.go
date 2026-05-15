package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type shellLeakAuthViolation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Reason  string `json:"reason"`
}

type shellLeakAuthResult struct {
	Violations []shellLeakAuthViolation `json:"violations"`
	Scanned    int                      `json:"files_scanned"`
}

// tomlAuthPatterns match shell-based auth in TOML [model_providers.*.auth] sections.
// All shell/bash/python patterns are blocked for auth commands.
var tomlAuthPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)command\s*=\s*"[^"]*\.sh"`),
	regexp.MustCompile(`(?i)command\s*=\s*"[^"]*\bbash\b`),
	regexp.MustCompile(`(?i)command\s*=\s*"[^"]*\bpython[23]?\b`),
}

var tomlAuthReasons = []string{
	"shell script (.sh) in auth command (TOML)",
	"bash invocation in auth command (TOML)",
	"python invocation in auth command (TOML)",
}

// jsonShellPatterns match only .sh scripts in JSON MCP configs.
// Python/Node MCP servers are legitimate; only bare .sh scripts are suspect.
var jsonShellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)"command"\s*:\s*"[^"]*\.sh"`),
}

var jsonShellReasons = []string{
	"shell script (.sh) in MCP/JSON command",
}

var doctorShellLeakAuthCmd = &cobra.Command{
	Use:   "shell-leak-auth",
	Short: "Scan IDE/CLI configs for forbidden shell-based auth commands",
	Long: `Scans ~/.codex/config.toml, ~/.cursor/mcp.json, and ~/.claude/mcp.json
for auth commands that use shell scripts, bash, or python helpers.
Shell-based auth causes timeouts, hook floods, and violates no-shell-leak rules.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		result := scanShellLeakAuth()
		if doctorOutputJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d config files\n", result.Scanned)
		if len(result.Violations) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "PASS shell-leak-auth: no shell-based auth found")
			return nil
		}
		for _, v := range result.Violations {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "FAIL %s:%d — %s\n  %s\n", v.File, v.Line, v.Reason, v.Content)
		}
		return fmt.Errorf("shell-leak-auth failed: %d violation(s)", len(result.Violations))
	},
}

func init() {
	doctorCmd.AddCommand(doctorShellLeakAuthCmd)
}

func scanShellLeakAuth() shellLeakAuthResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return shellLeakAuthResult{}
	}

	type scanTarget struct {
		path     string
		patterns []*regexp.Regexp
		reasons  []string
		isToml   bool
	}

	targets := []scanTarget{
		{filepath.Join(home, ".codex", "config.toml"), tomlAuthPatterns, tomlAuthReasons, true},
		{filepath.Join(home, ".cursor", "mcp.json"), jsonShellPatterns, jsonShellReasons, false},
		{filepath.Join(home, ".claude", "mcp.json"), jsonShellPatterns, jsonShellReasons, false},
		{filepath.Join(home, ".claude", "settings.json"), jsonShellPatterns, jsonShellReasons, false},
	}

	var result shellLeakAuthResult
	for _, t := range targets {
		data, err := os.ReadFile(t.path)
		if err != nil {
			continue
		}
		result.Scanned++

		lines := strings.Split(string(data), "\n")
		inAuthSection := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
				continue
			}

			if t.isToml {
				if strings.Contains(trimmed, "[") && strings.Contains(trimmed, ".auth]") {
					inAuthSection = true
					continue
				}
				if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed, ".auth]") {
					inAuthSection = false
					continue
				}
				if !inAuthSection {
					continue
				}
			}

			for j, pat := range t.patterns {
				if pat.MatchString(line) {
					result.Violations = append(result.Violations, shellLeakAuthViolation{
						File:    t.path,
						Line:    i + 1,
						Content: strings.TrimSpace(line),
						Reason:  t.reasons[j],
					})
				}
			}
		}
	}
	return result
}
