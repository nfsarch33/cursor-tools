// Package repomap generates structured .ai/repo-map.md files that describe a
// repository's directory tree, key files, and dependency summary. These maps
// help AI agents build context quickly without scanning the full filesystem.
package repomap

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config controls repo-map generation.
type Config struct {
	RootDir  string
	RepoName string
	MaxDepth int
}

var skipDirs = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	".ai":          true,
	"vendor":       true,
	"node_modules": true,
	"__pycache__":  true,
	".terraform":   true,
	"dist":         true,
	"build":        true,
}

var keyFileNames = map[string]bool{
	"go.mod":             true,
	"go.sum":             true,
	"Makefile":           true,
	"Dockerfile":         true,
	"docker-compose.yml": true,
	"package.json":       true,
	"tsconfig.json":      true,
	"README.md":          true,
	"AGENTS.md":          true,
	".env.example":       true,
	"Cargo.toml":         true,
	"pyproject.toml":     true,
	"pubspec.yaml":       true,
}

// Generate produces a structured repo-map markdown string.
func Generate(cfg Config) (string, error) {
	info, err := os.Stat(cfg.RootDir)
	if err != nil {
		return "", fmt.Errorf("root dir: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root path is not a directory: %s", cfg.RootDir)
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 3
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", cfg.RepoName))
	b.WriteString("## Directory Tree\n\n```\n")

	tree, keyFiles := walkTree(cfg.RootDir, cfg.MaxDepth)
	b.WriteString(tree)
	b.WriteString("```\n\n")

	if len(keyFiles) > 0 {
		b.WriteString("## Key Files\n\n")
		sort.Strings(keyFiles)
		for _, kf := range keyFiles {
			b.WriteString(fmt.Sprintf("- `%s`\n", kf))
		}
		b.WriteString("\n")
	}

	if deps := extractDeps(cfg.RootDir); deps != "" {
		b.WriteString("## Dependencies\n\n")
		b.WriteString(deps)
		b.WriteString("\n")
	}

	return b.String(), nil
}

type dirEntry struct {
	name  string
	isDir bool
}

func walkTree(root string, maxDepth int) (string, []string) {
	var b strings.Builder
	var keyFiles []string

	type walkItem struct {
		relPath string
		depth   int
		prefix  string
		entry   dirEntry
	}

	entries, err := readSortedDir(root)
	if err != nil {
		return "", nil
	}

	var stack []walkItem
	for i := len(entries) - 1; i >= 0; i-- {
		stack = append(stack, walkItem{
			relPath: entries[i].name,
			depth:   0,
			prefix:  "",
			entry:   entries[i],
		})
	}

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		display := item.entry.name
		if item.entry.isDir {
			display += "/"
		}
		b.WriteString(item.prefix + display + "\n")

		if keyFileNames[item.entry.name] {
			keyFiles = append(keyFiles, item.relPath)
		}

		if item.entry.isDir && item.depth < maxDepth {
			fullPath := filepath.Join(root, item.relPath)
			children, err := readSortedDir(fullPath)
			if err != nil {
				continue
			}
			childPrefix := item.prefix + "  "
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, walkItem{
					relPath: filepath.Join(item.relPath, children[i].name),
					depth:   item.depth + 1,
					prefix:  childPrefix,
					entry:   children[i],
				})
			}
		}
	}

	return b.String(), keyFiles
}

func readSortedDir(path string) ([]dirEntry, error) {
	osEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []dirEntry
	for _, e := range osEntries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && skipDirs[name] {
			continue
		}
		if e.IsDir() && skipDirs[name] {
			continue
		}
		if !e.IsDir() && shouldSkipFile(e) {
			continue
		}
		result = append(result, dirEntry{name: name, isDir: e.IsDir()})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].isDir != result[j].isDir {
			return result[i].isDir
		}
		return result[i].name < result[j].name
	})
	return result, nil
}

func shouldSkipFile(e fs.DirEntry) bool {
	name := e.Name()
	if strings.HasPrefix(name, ".") {
		return true
	}
	info, err := e.Info()
	if err != nil {
		return true
	}
	if info.Size() > 10<<20 {
		return true
	}
	return false
}

func extractDeps(root string) string {
	gomod := filepath.Join(root, "go.mod")
	if _, err := os.Stat(gomod); err != nil {
		return ""
	}

	f, err := os.Open(gomod)
	if err != nil {
		return ""
	}
	defer f.Close()

	var deps []string
	inRequire := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "require (") || strings.HasPrefix(line, "require(") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 && !strings.HasSuffix(parts[len(parts)-1], "indirect") {
				deps = append(deps, fmt.Sprintf("- `%s` %s", parts[0], parts[1]))
			}
		}
	}

	if len(deps) == 0 {
		return ""
	}
	return strings.Join(deps, "\n") + "\n"
}
