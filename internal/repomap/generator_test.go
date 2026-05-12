package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func makeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(base, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

func TestGenerate_BasicTree(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"go.mod":                   "module example.com/test\n\ngo 1.24\n",
		"cmd/main.go":              "package main\n\nfunc main() {}\n",
		"internal/foo/foo.go":      "package foo\n\nimport \"fmt\"\n\nfunc Hello() { fmt.Println(\"hello\") }\n",
		"internal/foo/foo_test.go": "package foo\n\nfunc TestHello(t *testing.T) {}\n",
		"README.md":                "# Test Project\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "test-repo",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	assert.Contains(t, out, "# test-repo")
	assert.Contains(t, out, "cmd/")
	assert.Contains(t, out, "internal/")
	assert.Contains(t, out, "go.mod")
	assert.Contains(t, out, "README.md")
}

func TestGenerate_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"a/b/c/d/deep.go": "package deep\n",
		"a/top.go":        "package a\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "depth-test",
		MaxDepth: 2,
	})
	require.NoError(t, err)

	assert.Contains(t, out, "a/")
	assert.Contains(t, out, "b/")
	assert.NotContains(t, out, "deep.go", "files beyond max depth should be omitted")
}

func TestGenerate_OutputUnder200Lines(t *testing.T) {
	root := t.TempDir()
	files := make(map[string]string)
	for i := range 50 {
		dir := filepath.Join("pkg", string(rune('a'+i%26)))
		files[filepath.Join(dir, "file.go")] = "package p\n"
	}
	makeTree(t, root, files)

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "large-repo",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	lines := strings.Count(out, "\n")
	assert.Less(t, lines, 200, "output should be under 200 lines")
}

func TestGenerate_SkipsDotDirs(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		".git/config":         "bare",
		".idea/workspace.xml": "<xml/>",
		"src/main.go":         "package main\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "dot-test",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	assert.NotContains(t, out, ".git")
	assert.NotContains(t, out, ".idea")
	assert.Contains(t, out, "src/")
}

func TestGenerate_SkipsVendorAndNodeModules(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"vendor/dep/dep.go":         "package dep\n",
		"node_modules/pkg/index.js": "module.exports = {};\n",
		"main.go":                   "package main\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "skip-vendor",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	assert.NotContains(t, out, "vendor/")
	assert.NotContains(t, out, "node_modules/")
	assert.Contains(t, out, "main.go")
}

func TestGenerate_IncludesKeyFiles(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"go.mod":                  "module example.com/proj\n\ngo 1.24\n",
		"Makefile":                "build:\n\tgo build ./...\n",
		"Dockerfile":              "FROM golang:1.24\n",
		".ai/repo-map.md":         "old content",
		"internal/api/handler.go": "package api\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "key-files",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	assert.Contains(t, out, "go.mod")
	assert.Contains(t, out, "Makefile")
	assert.Contains(t, out, "Dockerfile")
}

func TestGenerate_InvalidRoot(t *testing.T) {
	_, err := Generate(Config{
		RootDir:  "/nonexistent/path",
		RepoName: "nope",
		MaxDepth: 3,
	})
	assert.Error(t, err)
}

func TestGenerate_EmptyDir(t *testing.T) {
	root := t.TempDir()

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "empty",
		MaxDepth: 3,
	})
	require.NoError(t, err)
	assert.Contains(t, out, "# empty")
}

func TestGenerate_RootIsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello"), 0o644))

	_, err := Generate(Config{RootDir: f, RepoName: "nope", MaxDepth: 3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestGenerate_DefaultMaxDepth(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"a.go": "package main\n",
	})

	out, err := Generate(Config{RootDir: root, RepoName: "default-depth"})
	require.NoError(t, err)
	assert.Contains(t, out, "a.go")
}

func TestGenerate_DotFiles(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		".env":         "SECRET=val\n",
		".env.example": "SECRET=\n",
		"visible.go":   "package main\n",
	})

	out, err := Generate(Config{RootDir: root, RepoName: "dot-files", MaxDepth: 3})
	require.NoError(t, err)

	assert.NotContains(t, out, ".env\n")
	assert.Contains(t, out, "visible.go")
}

func TestGenerate_GoImportSummary(t *testing.T) {
	root := t.TempDir()
	makeTree(t, root, map[string]string{
		"go.mod":              "module example.com/proj\n\ngo 1.24\n\nrequire (\n\tgithub.com/spf13/cobra v1.8.0\n)\n",
		"main.go":             "package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/proj/internal/api\"\n)\n\nfunc main() { fmt.Println(api.Hello()) }\n",
		"internal/api/api.go": "package api\n\nfunc Hello() string { return \"hello\" }\n",
	})

	out, err := Generate(Config{
		RootDir:  root,
		RepoName: "import-test",
		MaxDepth: 3,
	})
	require.NoError(t, err)

	assert.Contains(t, out, "## Dependencies")
	assert.Contains(t, out, "cobra")
}
