//go:build gui

// Command soholink is the single unified GUI entry point for the SoHoLINK node.
//
// First run (no configured node name): opens the Setup Wizard.
// Subsequent runs (configured node):   opens the full operator Dashboard.
//
// Build:
//
//	go build -tags gui -o soholink ./cmd/soholink/
package main

import (
	"io/fs"
	"log"
	"os"

	soholink "github.com/NetworkTheoryAppliedResearchInstitute/soholink"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/app"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/cli"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/gui/dashboard"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/policy"
)

// Build-time variables set by -ldflags.
var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Register embedded defaults so config.Load() can seed values.
	config.SetDefaultConfig(soholink.DefaultConfigYAML)
	cli.SetDefaultPolicy(soholink.DefaultPolicyRego)

	// Register the embedded .rego policies so the policy engine can start
	// without an external policy directory — mirrors SetDefaultConfig above.
	// fs.Sub strips the "configs/policies" prefix so *.rego files are at root.
	if sub, err := fs.Sub(soholink.PoliciesFS, "configs/policies"); err == nil {
		policy.SetEmbeddedFS(sub)
	}

	// If any non-GUI subcommand was requested, delegate to the CLI and exit.
	// This lets operators run `soholink status` or `soholink start` from the
	// same binary without the GUI appearing.
	if len(os.Args) > 1 {
		cli.Execute(version, commit, buildTime)
		return
	}

	// ── GUI path ──────────────────────────────────────────────────────────────

	// Try to load the node configuration.
	cfgFile := os.Getenv("SOHOLINK_CONFIG")
	cfg, err := config.Load(cfgFile)
	if err != nil {
		// No readable config → run the wizard with nothing pre-populated.
		log.Printf("[soholink] config load failed (%v); launching Setup Wizard", err)
		dashboard.RunSetupWizard(nil, nil)
		return
	}

	// First-run detection: wizard hasn't been completed if Node.Name is empty.
	if cfg.Node.Name == "" {
		log.Printf("[soholink] no node name configured; launching Setup Wizard")
		dashboard.RunSetupWizard(cfg, nil)
		return
	}

	// Attempt to initialise all subsystems.
	application, err := app.New(cfg)
	if err != nil {
		// Subsystem init failed — likely a new install or broken config.
		// Fall back to the wizard so the operator can reconfigure.
		log.Printf("[soholink] app init failed (%v); launching Setup Wizard", err)
		dashboard.RunSetupWizard(cfg, nil)
		return
	}

	// Propagate build-time version info into the app (HTTP API + updater).
	application.SetVersion(version, commit, buildTime)

	// Everything is ready — open the full dashboard.
	dashboard.RunDashboard(application)
}
