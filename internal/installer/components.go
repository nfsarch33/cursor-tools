package installer

import (
	"os"
	"path/filepath"
)

func DefaultRegistry(repoBase, outputBase string) *Registry {
	r := NewRegistry()

	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}

	runsDir := filepath.Join(home, "runs")
	if outputBase != "" {
		runsDir = outputBase
	}

	cursorToolsRepo := filepath.Join(home, "cursor-tools")
	if repoBase != "" {
		cursorToolsRepo = repoBase
	}

	r.Register("cursor-tools", &GoBuildInstaller{
		ComponentName: "cursor-tools",
		RepoPath:      cursorToolsRepo,
		BinaryName:    "cursor-tools",
		OutputDir:     runsDir,
		MainPkg:       ".",
		VersionFlag:   "version",
	})

	r.Register("runx", &GoBuildInstaller{
		ComponentName: "runx",
		RepoPath:      filepath.Join(home, "Code", "runx"),
		BinaryName:    "runx",
		OutputDir:     runsDir,
		MainPkg:       ".",
		VersionFlag:   "version",
	})

	r.Register("mem0-mcp-go", &GoBuildInstaller{
		ComponentName: "mem0-mcp-go",
		RepoPath:      filepath.Join(home, "Code", "mem0-mcp-go"),
		BinaryName:    "mem0-mcp-go",
		OutputDir:     runsDir,
		MainPkg:       ".",
		VersionFlag:   "--version",
	})

	r.Register("ironclaw-mcp", &GoBuildInstaller{
		ComponentName: "ironclaw-mcp",
		RepoPath:      filepath.Join(home, "Code", "ironclaw-mcp"),
		BinaryName:    "ironclaw-mcp",
		OutputDir:     runsDir,
		MainPkg:       ".",
		VersionFlag:   "--version",
	})

	return r
}
