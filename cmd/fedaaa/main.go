package main

import (
	soholink "github.com/NetworkTheoryAppliedResearchInstitute/soholink"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/cli"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
)

// Build-time variables set via -ldflags
var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// ── Startup wiring ──────────────────────────────────────────────────────
	// These calls must run before any subsystem (app.New, cli.Execute) starts.
	// Add a line here — and to internal/audit/wiring_test.go requiredMainCalls
	// — whenever you introduce a new package-level registration function.
	//
	//   config.SetDefaultConfig  → seeds Viper defaults from the embedded YAML
	//   cli.SetDefaultPolicy     → stores default .rego bytes for `install` cmd
	//
	// NOTE: policy.SetEmbeddedFS is intentionally absent. The embedded .rego FS
	// is now wired inside app.New() via an explicit argument to policy.NewEngine.
	config.SetDefaultConfig(soholink.DefaultConfigYAML)
	cli.SetDefaultPolicy(soholink.DefaultPolicyRego)

	cli.Execute(version, commit, buildTime)
}
