package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/installer"
)

var (
	installDryRun    bool
	installOutputDir string
	installRepoBase  string
)

var installCmd = &cobra.Command{
	Use:   "install <component>",
	Short: "Build and install a Go component binary",
	Long: `Build a component from source using go build and place the binary
in the target directory. Available components: cursor-tools, runx,
mem0-mcp-go, ironclaw-mcp.`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

var installListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available components for installation",
	Run: func(cmd *cobra.Command, _ []string) {
		reg := installer.DefaultRegistry(installRepoBase, installOutputDir)
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "Available components:")
		for _, name := range reg.List() {
			inst, _ := reg.Get(name)
			plan := inst.Plan()
			fmt.Fprintf(out, "  %-20s -> %s\n", name, plan.OutputPath)
		}
	},
}

func init() {
	installCmd.AddCommand(installListCmd)
	installCmd.PersistentFlags().BoolVar(&installDryRun, "dry-run", false, "show what would happen without building")
	installCmd.PersistentFlags().StringVar(&installOutputDir, "output-dir", "", "override output directory (default: ~/runs/)")
	installCmd.PersistentFlags().StringVar(&installRepoBase, "repo-base", "", "override repo base path")
}

func runInstall(_ *cobra.Command, args []string) error {
	component := args[0]
	reg := installer.DefaultRegistry(installRepoBase, installOutputDir)

	inst, ok := reg.Get(component)
	if !ok {
		return fmt.Errorf("unknown component %q; run 'cursor-tools install list' to see options", component)
	}

	plan := inst.Plan()
	if installDryRun {
		fmt.Printf("[DRY-RUN] %s\n", plan.Component)
		fmt.Printf("  steps: %s\n", plan.Steps)
		fmt.Printf("  output: %s\n", plan.OutputPath)
		return nil
	}

	fmt.Printf("[INSTALL] building %s...\n", component)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := inst.Install(ctx)
	if err != nil {
		return fmt.Errorf("install %s: %w", component, err)
	}

	if result.Success {
		fmt.Printf("[OK] %s installed at %s\n", result.Component, result.Path)
		if result.Version != "" {
			fmt.Printf("  version: %s\n", result.Version)
		}
	} else {
		fmt.Printf("[FAIL] %s: %s\n", result.Component, result.Message)
		return fmt.Errorf("install failed for %s", component)
	}
	return nil
}
