package docsaudit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestAudit_ComponentWithDoc(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, ".cursor", "rules", "example.mdc"), "rule content")
	writeFile(t, filepath.Join(root, "sop", "example.md"), "SOP for example rule")

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "example-rule",
		Kind:     KindRule,
		Path:     filepath.Join(".cursor", "rules", "example.mdc"),
		DocPath:  filepath.Join("sop", "example.md"),
		DocCheck: DocCheckExists,
	})

	report := Audit(root, reg)
	assert.True(t, report.OK(), "expected no findings for fully documented component")
	assert.Empty(t, report.Findings)
}

func TestAudit_ComponentWithoutDoc(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, ".cursor", "rules", "orphan.mdc"), "rule content")

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "orphan-rule",
		Kind:     KindRule,
		Path:     filepath.Join(".cursor", "rules", "orphan.mdc"),
		DocPath:  filepath.Join("sop", "orphan.md"),
		DocCheck: DocCheckExists,
	})

	report := Audit(root, reg)
	assert.False(t, report.OK(), "expected a finding for missing doc")
	require.Len(t, report.Findings, 1)
	assert.Equal(t, "orphan-rule", report.Findings[0].Component)
	assert.False(t, report.Findings[0].HasDoc)
}

func TestAudit_FreshnessCheck(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, "bin", "cursor-tools"), "binary")
	writeFile(t, filepath.Join(root, "docs", "install-cursor-tools.md"), "install guide")

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "cursor-tools",
		Kind:     KindBinary,
		Path:     filepath.Join("bin", "cursor-tools"),
		DocPath:  filepath.Join("docs", "install-cursor-tools.md"),
		DocCheck: DocCheckExists,
	})

	report := Audit(root, reg)
	assert.True(t, report.OK())
	require.Len(t, report.Results, 1)
	assert.NotEmpty(t, report.Results[0].Freshness, "freshness should be populated")
}

func TestAudit_InstallerCheck(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, "bin", "runx"), "binary")
	writeFile(t, filepath.Join(root, "docs", "install-runx.md"), "install guide")

	reg := NewRegistry()
	reg.Add(Component{
		Name:         "runx",
		Kind:         KindBinary,
		Path:         filepath.Join("bin", "runx"),
		DocPath:      filepath.Join("docs", "install-runx.md"),
		DocCheck:     DocCheckExists,
		HasInstaller: true,
	})

	report := Audit(root, reg)
	assert.True(t, report.OK())
	require.Len(t, report.Results, 1)
	assert.True(t, report.Results[0].HasInstaller)
}

func TestAudit_SkillWithoutTrigger(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, "skills", "my-skill", "SKILL.md"), "This skill does stuff.")

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "my-skill",
		Kind:     KindSkill,
		Path:     filepath.Join("skills", "my-skill", "SKILL.md"),
		DocPath:  filepath.Join("skills", "my-skill", "SKILL.md"),
		DocCheck: DocCheckSkillTrigger,
	})

	report := Audit(root, reg)
	assert.False(t, report.OK())
	require.Len(t, report.Findings, 1)
	assert.Contains(t, report.Findings[0].Message, "trigger")
}

func TestAudit_SkillWithTrigger(t *testing.T) {
	root := tempDir(t)
	writeFile(t, filepath.Join(root, "skills", "my-skill", "SKILL.md"), "Use when: building things\nTrigger: on keyword match")

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "my-skill",
		Kind:     KindSkill,
		Path:     filepath.Join("skills", "my-skill", "SKILL.md"),
		DocPath:  filepath.Join("skills", "my-skill", "SKILL.md"),
		DocCheck: DocCheckSkillTrigger,
	})

	report := Audit(root, reg)
	assert.True(t, report.OK())
}

func TestAudit_MissingComponent(t *testing.T) {
	root := tempDir(t)

	reg := NewRegistry()
	reg.Add(Component{
		Name:     "ghost",
		Kind:     KindBinary,
		Path:     filepath.Join("bin", "ghost"),
		DocPath:  filepath.Join("docs", "ghost.md"),
		DocCheck: DocCheckExists,
	})

	report := Audit(root, reg)
	assert.Len(t, report.Results, 1)
	assert.False(t, report.Results[0].ComponentExists)
}

func TestReport_JSON(t *testing.T) {
	report := Report{
		Results: []Result{
			{
				Component:       "test",
				HasDoc:          true,
				HasInstaller:    false,
				DocPath:         "docs/test.md",
				ComponentExists: true,
			},
		},
	}
	data, err := report.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"component"`)
	assert.Contains(t, string(data), `"has_doc"`)
	assert.Contains(t, string(data), `"has_installer"`)
}

func TestAutoScan_Rules(t *testing.T) {
	root := tempDir(t)
	rulesDir := filepath.Join(root, "cursor-config", "rules")
	writeFile(t, filepath.Join(rulesDir, "alpha.mdc"), "rule alpha")
	writeFile(t, filepath.Join(rulesDir, "beta.mdc"), "rule beta")

	reg := AutoScanRules(root)
	assert.Len(t, reg.Components(), 2)
}

func TestAutoScan_Skills(t *testing.T) {
	root := tempDir(t)
	skillsDir := filepath.Join(root, "skills")
	writeFile(t, filepath.Join(skillsDir, "foo", "SKILL.md"), "Use when: doing foo")
	writeFile(t, filepath.Join(skillsDir, "bar", "SKILL.md"), "Trigger: on bar keyword")

	reg := AutoScanSkills(root, "skills")
	assert.Len(t, reg.Components(), 2)
}
