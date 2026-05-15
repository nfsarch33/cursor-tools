package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandoffFilename(t *testing.T) {
	name := handoffFilename("v454-1")
	if !strings.HasSuffix(name, "-v454-1-handoff.md") {
		t.Errorf("unexpected filename: %s", name)
	}
}

func TestHandoffFilename_SanitizesSlashes(t *testing.T) {
	name := handoffFilename("phase/0/shell-auth")
	if strings.Contains(name, "/") {
		t.Errorf("filename should not contain slashes: %s", name)
	}
}

func TestHandoffInit_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	kbDir := filepath.Join(tmp, "Code", "global-kb")
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handoffTodoID = "test-todo-1"
	handoffRepo = "cursor-tools"
	handoffBranch = "feat/test"
	handoffContent = "Test handoff creation"

	if err := runHandoffInit(nil, nil); err != nil {
		t.Fatalf("runHandoffInit failed: %v", err)
	}

	dir := filepath.Join(kbDir, "session-handoffs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read handoff dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 handoff file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "test-todo-1") {
		t.Error("handoff missing todo ID")
	}
	if !strings.Contains(content, "IN PROGRESS") {
		t.Error("handoff missing IN PROGRESS status")
	}
	if !strings.Contains(content, "cursor-tools") {
		t.Error("handoff missing repo name")
	}
}

func TestHandoffFinalize_UpdatesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	kbDir := filepath.Join(tmp, "Code", "global-kb")
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handoffTodoID = "test-todo-2"
	handoffContent = "Test finalization"
	handoffRepo = "global-kb"
	handoffBranch = "main"

	if err := runHandoffInit(nil, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	handoffEvidence = "All 8 tests pass; binary rebuilt"
	if err := runHandoffFinalize(nil, nil); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	dir := filepath.Join(kbDir, "session-handoffs")
	entries, _ := os.ReadDir(dir)
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if strings.Contains(content, "IN PROGRESS") {
		t.Error("finalized handoff still shows IN PROGRESS")
	}
	if !strings.Contains(content, "COMPLETED") {
		t.Error("finalized handoff missing COMPLETED status")
	}
	if !strings.Contains(content, "All 8 tests pass") {
		t.Error("finalized handoff missing evidence")
	}
}

func TestHandoffInit_RequiresTodoID(t *testing.T) {
	handoffTodoID = ""
	err := runHandoffInit(nil, nil)
	if err == nil {
		t.Error("expected error for missing todo-id")
	}
}

func TestHandoffFinalize_RequiresTodoID(t *testing.T) {
	handoffTodoID = ""
	err := runHandoffFinalize(nil, nil)
	if err == nil {
		t.Error("expected error for missing todo-id")
	}
}

func TestHandoffFinalize_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	handoffTodoID = "nonexistent-todo"
	err := runHandoffFinalize(nil, nil)
	if err == nil {
		t.Error("expected error for missing handoff file")
	}
}
