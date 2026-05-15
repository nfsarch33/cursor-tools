package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/fleethealth"
	"github.com/spf13/cobra"
)

var fleetHealthCmd = &cobra.Command{
	Use:   "fleet-health",
	Short: "Fleet health daemon and status",
	Long:  "Run fleet health probes (SSH, Docker, Mem0) across nodes and write NDJSON results.",
}

var fleetHealthRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the fleet-health daemon loop",
	Long: `Periodically probe all configured fleet nodes via SSH canary,
Docker status, and Mem0 /healthz checks. Results are written as
NDJSON to ~/logs/runx/fleet-health.ndjson.

Runs until interrupted (SIGINT/SIGTERM).`,
	RunE: runFleetHealthDaemon,
}

var fleetHealthStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show latest fleet health from NDJSON log",
	RunE:  runFleetHealthStatus,
}

var (
	fleetHealthInterval string
	fleetHealthOutput   string
)

func init() {
	fleetHealthRunCmd.Flags().StringVar(&fleetHealthInterval, "interval", "5m", "Probe interval (e.g. 5m, 30s)")
	fleetHealthRunCmd.Flags().StringVar(&fleetHealthOutput, "output", "", "NDJSON output path (default: ~/logs/runx/fleet-health.ndjson)")

	fleetHealthCmd.AddCommand(fleetHealthRunCmd)
	fleetHealthCmd.AddCommand(fleetHealthStatusCmd)
}

func defaultFleetNodes() []fleethealth.Node {
	return []fleethealth.Node{
		{
			Name:      "win1",
			Type:      fleethealth.NodeTypeWindows,
			SSHAlias:  "win1-via-oracle",
			State:     fleethealth.NodeStateActive,
			HasDocker: false,
			HasMem0:   false,
		},
		{
			Name:      "wsl1",
			Type:      fleethealth.NodeTypeWSL,
			SSHAlias:  "wsl1-via-oracle",
			State:     fleethealth.NodeStateActive,
			HasDocker: true,
			HasMem0:   true,
		},
	}
}

func runFleetHealthDaemon(cmd *cobra.Command, args []string) error {
	interval, err := time.ParseDuration(fleetHealthInterval)
	if err != nil {
		return fmt.Errorf("parse interval: %w", err)
	}

	cfg := fleethealth.DefaultConfig()
	cfg.ProbeInterval = interval
	cfg.Nodes = defaultFleetNodes()

	if fleetHealthOutput != "" {
		cfg.OutputPath = fleetHealthOutput
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return fleethealth.Run(ctx, cfg)
}

func runFleetHealthStatus(cmd *cobra.Command, args []string) error {
	cfg := fleethealth.DefaultConfig()
	path := cfg.OutputPath
	if fleetHealthOutput != "" {
		path = fleetHealthOutput
	}

	latest, err := fleethealth.ReadLatest(path)
	if err != nil {
		return fmt.Errorf("read fleet health: %w", err)
	}

	table := fleethealth.FormatTable(latest)
	fmt.Fprint(cmd.OutOrStdout(), table)

	for _, r := range latest {
		if r.Overall == fleethealth.ProbeFail {
			slog.Warn("fleet node degraded", "node", r.Node)
		}
	}
	return nil
}
