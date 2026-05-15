package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register("alpha", &stubInstaller{name: "alpha"})
	reg.Register("beta", &stubInstaller{name: "beta"})

	names := reg.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()
	s := &stubInstaller{name: "alpha"}
	reg.Register("alpha", s)

	got, ok := reg.Get("alpha")
	require.True(t, ok)
	assert.Equal(t, s, got)

	_, ok = reg.Get("missing")
	assert.False(t, ok)
}

func TestInstaller_Install(t *testing.T) {
	dir := t.TempDir()
	s := &stubInstaller{name: "test-tool", outputDir: dir}

	result, err := s.Install(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test-tool", result.Component)
}

func TestInstaller_Verify(t *testing.T) {
	s := &stubInstaller{name: "test-tool", verifyOK: true}

	ok, ver := s.Verify(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "v1.0.0-stub", ver)
}

func TestGoBuildInstaller_Plan(t *testing.T) {
	inst := &GoBuildInstaller{
		ComponentName: "cursor-tools",
		RepoPath:      "/fake/repo",
		BinaryName:    "cursor-tools",
		OutputDir:     "/fake/out",
		MainPkg:       "./cmd/cursor-tools",
	}

	plan := inst.Plan()
	assert.Equal(t, "cursor-tools", plan.Component)
	assert.Contains(t, plan.Steps, "go build")
	assert.Contains(t, plan.OutputPath, "cursor-tools")
}

func TestGoBuildInstaller_DryRun(t *testing.T) {
	dir := t.TempDir()
	inst := &GoBuildInstaller{
		ComponentName: "cursor-tools",
		RepoPath:      dir,
		BinaryName:    "cursor-tools",
		OutputDir:     filepath.Join(dir, "out"),
		MainPkg:       ".",
	}

	plan := inst.Plan()
	assert.False(t, plan.DryRun)
}

func TestDefaultRegistry_HasAllComponents(t *testing.T) {
	reg := DefaultRegistry(t.TempDir(), t.TempDir())
	expected := []string{"cursor-tools", "runx", "mem0-mcp-go", "ironclaw-mcp"}
	for _, name := range expected {
		_, ok := reg.Get(name)
		assert.True(t, ok, "expected component %q in default registry", name)
	}
}

func TestGoBuildInstaller_InstallCreatesOutputDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "nested", "bin")

	writeGoMain(t, filepath.Join(dir, "src"))

	inst := &GoBuildInstaller{
		ComponentName: "test-bin",
		RepoPath:      filepath.Join(dir, "src"),
		BinaryName:    "test-bin",
		OutputDir:     outDir,
		MainPkg:       ".",
	}

	result, err := inst.Install(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)

	_, statErr := os.Stat(filepath.Join(outDir, "test-bin"))
	assert.NoError(t, statErr, "binary should exist in output dir")
}

func TestGoBuildInstaller_VerifyAfterInstall(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "bin")

	writeGoMain(t, filepath.Join(dir, "src"))

	inst := &GoBuildInstaller{
		ComponentName: "test-bin",
		RepoPath:      filepath.Join(dir, "src"),
		BinaryName:    "test-bin",
		OutputDir:     outDir,
		MainPkg:       ".",
		VersionFlag:   "--version",
	}

	result, err := inst.Install(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)

	ok, ver := inst.Verify(context.Background())
	assert.True(t, ok, "binary should be verifiable")
	assert.NotEmpty(t, ver)
}

// --- helpers ---

type stubInstaller struct {
	name      string
	outputDir string
	verifyOK  bool
}

func (s *stubInstaller) Install(_ context.Context) (InstallResult, error) {
	return InstallResult{
		Component: s.name,
		Success:   true,
		Path:      filepath.Join(s.outputDir, s.name),
	}, nil
}

func (s *stubInstaller) Verify(_ context.Context) (bool, string) {
	if s.verifyOK {
		return true, "v1.0.0-stub"
	}
	return false, ""
}

func (s *stubInstaller) Plan() InstallPlan {
	return InstallPlan{
		Component:  s.name,
		Steps:      "stub install",
		OutputPath: filepath.Join(s.outputDir, s.name),
	}
}

func writeGoMain(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))

	main := `package main

import "fmt"

func main() { fmt.Println("test-bin v0.0.1-test") }
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644))

	gomod := "module test-bin\n\ngo 1.25.0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644))
}
