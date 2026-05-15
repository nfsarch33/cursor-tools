package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/docsaudit"
)

var (
	docsAuditRoot string
	docsAuditJSON bool
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Documentation management commands",
}

var docsAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit documentation coverage for all known components",
	Long: `Scan rules, skills, binaries, LaunchAgents, and daemons for matching
documentation. Reports gaps as structured JSON or human-readable text.`,
	RunE: runDocsAudit,
}

func init() {
	docsCmd.AddCommand(docsAuditCmd)
	docsAuditCmd.Flags().StringVar(&docsAuditRoot, "root", "", "audit root directory (defaults to global-kb path)")
	docsAuditCmd.Flags().BoolVar(&docsAuditJSON, "json", false, "output as JSON")
}

func runDocsAudit(_ *cobra.Command, _ []string) error {
	root := docsAuditRoot
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("docs audit: %w", err)
		}
		root = home + "/Code/global-kb"
	}

	reg := docsaudit.DefaultRegistry()
	reg.Merge(docsaudit.AutoScanRules(root))
	reg.Merge(docsaudit.AutoScanSkills(root, "skills"))
	reg.Merge(docsaudit.AutoScanLaunchAgents(root))

	report := docsaudit.Audit(root, reg)

	if docsAuditJSON {
		data, err := report.JSON()
		if err != nil {
			return fmt.Errorf("docs audit: marshal: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	out := os.Stdout
	if report.OK() {
		fmt.Fprintln(out, "[PASS] docs audit: all components documented")
	} else {
		fmt.Fprintln(out, "[FAIL] docs audit: documentation gaps found")
	}
	for _, r := range report.Results {
		status := "OK"
		if !r.HasDoc {
			status = "MISSING"
		}
		if !r.ComponentExists {
			status = "NOT_FOUND"
		}
		installer := ""
		if r.HasInstaller {
			installer = " [installer]"
		}
		fmt.Fprintf(out, "  [%s] %s -> %s%s\n", status, r.Component, r.DocPath, installer)
	}
	for _, f := range report.Findings {
		fmt.Fprintf(out, "  [GAP] %s: %s\n", f.Component, f.Message)
	}

	if !report.OK() {
		return fmt.Errorf("docs audit: %d gap(s) found", len(report.Findings))
	}
	return nil
}
