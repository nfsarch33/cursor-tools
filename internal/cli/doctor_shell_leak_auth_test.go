package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanShellLeakAuth_CleanConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	toml := filepath.Join(tmp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(toml), 0o755); err != nil {
		t.Fatal(err)
	}
	clean := `model_provider = "zd-ai-gateway"
model = "gpt-5.4"

[model_providers.zd-ai-gateway.auth]
env_key = "OPENAI_API_KEY"
`
	if err := os.WriteFile(toml, []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if result.Scanned != 1 {
		t.Errorf("expected 1 file scanned, got %d", result.Scanned)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations on clean config, got %d: %v", len(result.Violations), result.Violations)
	}
}

func TestScanShellLeakAuth_ShellScript(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	toml := filepath.Join(tmp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(toml), 0o755); err != nil {
		t.Fatal(err)
	}
	dirty := `[model_providers.zd-ai-gateway.auth]
command = "/home/user/.codex/scripts/op-fetch-openai-key.sh"
timeout_ms = 5000
`
	if err := os.WriteFile(toml, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation for .sh command, got %d", len(result.Violations))
	}
	v := result.Violations[0]
	if v.Line != 2 {
		t.Errorf("expected violation on line 2, got %d", v.Line)
	}
	if v.Reason != "shell script (.sh) in auth command (TOML)" {
		t.Errorf("unexpected reason: %s", v.Reason)
	}
}

func TestScanShellLeakAuth_BashCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	toml := filepath.Join(tmp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(toml), 0o755); err != nil {
		t.Fatal(err)
	}
	dirty := `[model_providers.custom.auth]
command = "bash -c 'op read op://vault/item/field'"
`
	if err := os.WriteFile(toml, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation for bash command, got %d", len(result.Violations))
	}
}

func TestScanShellLeakAuth_MCPJson(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	mcpDir := filepath.Join(tmp, ".cursor")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirty := `{
  "mcpServers": {
    "bad-server": {
      "command": "/path/to/script.sh",
      "args": ["--flag"]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(mcpDir, "mcp.json"), []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation for MCP .sh command, got %d", len(result.Violations))
	}
}

func TestScanShellLeakAuth_PythonMCPServerIsAllowed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	mcpDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legitimate := `{
  "mcpServers": {
    "pdf-handler": {
      "command": "/path/to/.venv/bin/python",
      "args": ["-m", "pdf_mcp.server"]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(mcpDir, "mcp.json"), []byte(legitimate), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations for legitimate Python MCP server, got %d", len(result.Violations))
	}
}

func TestScanShellLeakAuth_TOMLAuthPythonBlocked(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	toml := filepath.Join(tmp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(toml), 0o755); err != nil {
		t.Fatal(err)
	}
	dirty := `[model_providers.custom.auth]
command = "python3 /path/to/secret_fetcher.py"
`
	if err := os.WriteFile(toml, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation for python auth command in TOML, got %d", len(result.Violations))
	}
}

func TestScanShellLeakAuth_CommentsSkipped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	toml := filepath.Join(tmp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(toml), 0o755); err != nil {
		t.Fatal(err)
	}
	withComments := `# command = "/path/to/old-script.sh"
[model_providers.clean.auth]
env_key = "API_KEY"
`
	if err := os.WriteFile(toml, []byte(withComments), 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanShellLeakAuth()
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations for commented lines, got %d", len(result.Violations))
	}
}

func TestScanShellLeakAuth_MissingFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	result := scanShellLeakAuth()
	if result.Scanned != 0 {
		t.Errorf("expected 0 files scanned when none exist, got %d", result.Scanned)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations when no files, got %d", len(result.Violations))
	}
}
