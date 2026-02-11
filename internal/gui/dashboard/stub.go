//go:build !gui

// Package dashboard provides operator-facing GUI views for SoHoLINK.
// This stub file is compiled when the gui build tag is NOT present.
package dashboard

import (
	"fmt"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/app"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// RunSetupWizard is a stub that returns an error when GUI support is not compiled in.
func RunSetupWizard(cfg *config.Config, s *store.Store) {
	fmt.Println("Error: GUI support not compiled in. Build with: make build-gui")
	fmt.Println("Or run the CLI installer: fedaaa install")
}

// RunDashboard is a stub that returns an error when GUI support is not compiled in.
func RunDashboard(application *app.App) {
	fmt.Println("Error: GUI support not compiled in. Build with: make build-gui")
	fmt.Println("Use the CLI to manage your node: fedaaa --help")
}
