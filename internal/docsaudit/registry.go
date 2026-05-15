package docsaudit

import "path/filepath"

type ComponentKind string

const (
	KindRule         ComponentKind = "rule"
	KindBinary       ComponentKind = "binary"
	KindSkill        ComponentKind = "skill"
	KindLaunchAgent  ComponentKind = "launch_agent"
	KindDaemon       ComponentKind = "daemon"
	KindCLI          ComponentKind = "cli"
)

type DocCheckMode int

const (
	DocCheckExists       DocCheckMode = iota
	DocCheckSkillTrigger              // doc must contain trigger-condition keywords
)

type Component struct {
	Name         string
	Kind         ComponentKind
	Path         string // relative path from audit root
	DocPath      string // expected doc path relative to audit root
	DocCheck     DocCheckMode
	HasInstaller bool
}

type Registry struct {
	components []Component
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Add(c Component) {
	r.components = append(r.components, c)
}

func (r *Registry) Components() []Component {
	return r.components
}

func (r *Registry) Merge(other *Registry) {
	r.components = append(r.components, other.components...)
}

func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Add(Component{
		Name:         "cursor-tools",
		Kind:         KindBinary,
		Path:         filepath.Join("bin", "cursor-tools"),
		DocPath:      filepath.Join("docs", "install-cursor-tools.md"),
		DocCheck:     DocCheckExists,
		HasInstaller: true,
	})
	r.Add(Component{
		Name:         "runx",
		Kind:         KindBinary,
		Path:         filepath.Join("bin", "runx"),
		DocPath:      filepath.Join("docs", "install-runx.md"),
		DocCheck:     DocCheckExists,
		HasInstaller: true,
	})
	r.Add(Component{
		Name:         "mem0-mcp-go",
		Kind:         KindBinary,
		Path:         filepath.Join("bin", "mem0-mcp-go"),
		DocPath:      filepath.Join("docs", "install-mem0-mcp-go.md"),
		DocCheck:     DocCheckExists,
		HasInstaller: true,
	})
	r.Add(Component{
		Name:         "ironclaw-mcp",
		Kind:         KindBinary,
		Path:         filepath.Join("bin", "ironclaw-mcp"),
		DocPath:      filepath.Join("docs", "install-ironclaw-mcp.md"),
		DocCheck:     DocCheckExists,
		HasInstaller: true,
	})

	return r
}
