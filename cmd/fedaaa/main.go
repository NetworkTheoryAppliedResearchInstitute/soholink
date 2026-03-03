package main

import (
	"io/fs"
	"log"

	soholink "github.com/NetworkTheoryAppliedResearchInstitute/soholink"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/cli"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/policy"
)

// Build-time variables set via -ldflags
var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	config.SetDefaultConfig(soholink.DefaultConfigYAML)
	cli.SetDefaultPolicy(soholink.DefaultPolicyRego)

	// Register embedded policies so the engine works with zero external files.
	// fs.Sub strips the "configs/policies" prefix; the engine sees "*.rego" directly.
	sub, err := fs.Sub(soholink.PoliciesFS, "configs/policies")
	if err != nil {
		log.Fatalf("failed to sub embedded policies FS: %v", err)
	}
	policy.SetEmbeddedFS(sub)

	cli.Execute(version, commit, buildTime)
}
