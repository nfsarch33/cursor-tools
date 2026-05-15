package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Installer interface {
	Install(ctx context.Context) (InstallResult, error)
	Verify(ctx context.Context) (ok bool, version string)
	Plan() InstallPlan
}

type InstallResult struct {
	Component string `json:"component"`
	Success   bool   `json:"success"`
	Path      string `json:"path"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message,omitempty"`
}

type InstallPlan struct {
	Component  string `json:"component"`
	Steps      string `json:"steps"`
	OutputPath string `json:"output_path"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

type Registry struct {
	installers map[string]Installer
}

func NewRegistry() *Registry {
	return &Registry{installers: make(map[string]Installer)}
}

func (r *Registry) Register(name string, inst Installer) {
	r.installers[name] = inst
}

func (r *Registry) Get(name string) (Installer, bool) {
	inst, ok := r.installers[name]
	return inst, ok
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.installers))
	for name := range r.installers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type GoBuildInstaller struct {
	ComponentName string
	RepoPath      string
	BinaryName    string
	OutputDir     string
	MainPkg       string
	VersionFlag   string // e.g. "--version" or "version"
	LDFlags       string
}

func (g *GoBuildInstaller) Install(ctx context.Context) (InstallResult, error) {
	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return InstallResult{Component: g.ComponentName}, fmt.Errorf("create output dir: %w", err)
	}

	outputPath := filepath.Join(g.OutputDir, g.BinaryName)
	args := []string{"build", "-o", outputPath}
	if g.LDFlags != "" {
		args = append(args, "-ldflags", g.LDFlags)
	}
	args = append(args, g.MainPkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = g.RepoPath
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return InstallResult{
			Component: g.ComponentName,
			Success:   false,
			Message:   fmt.Sprintf("go build failed: %s", strings.TrimSpace(string(out))),
		}, fmt.Errorf("go build: %w", err)
	}

	ok, ver := g.Verify(ctx)
	return InstallResult{
		Component: g.ComponentName,
		Success:   ok,
		Path:      outputPath,
		Version:   ver,
	}, nil
}

func (g *GoBuildInstaller) Verify(ctx context.Context) (bool, string) {
	binaryPath := filepath.Join(g.OutputDir, g.BinaryName)
	if _, err := os.Stat(binaryPath); err != nil {
		return false, ""
	}

	if g.VersionFlag == "" {
		return true, "(no version flag)"
	}

	cmd := exec.CommandContext(ctx, binaryPath, g.VersionFlag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, "(version check failed)"
	}
	return true, strings.TrimSpace(string(out))
}

func (g *GoBuildInstaller) Plan() InstallPlan {
	return InstallPlan{
		Component:  g.ComponentName,
		Steps:      fmt.Sprintf("go build -o %s/%s %s", g.OutputDir, g.BinaryName, g.MainPkg),
		OutputPath: filepath.Join(g.OutputDir, g.BinaryName),
	}
}
