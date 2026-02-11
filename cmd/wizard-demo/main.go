package main

import (
	"fmt"
	"log"
	"os"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/wizard"
)

func main() {
	fmt.Println()
	fmt.Println("Starting SoHoLINK Deployment Wizard Demo...")
	fmt.Println()

	if err := wizard.DemoWizardFlow(); err != nil {
		log.Fatalf("❌ Wizard demo failed: %v", err)
		os.Exit(1)
	}

	fmt.Println("\n✨ Wizard demo completed successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review generated configuration files")
	fmt.Println("  2. Configure firewall (open ports 1812-1813)")
	fmt.Println("  3. Start SoHoLINK service")
	fmt.Println("  4. Begin accepting contracts and earning! 💰")
	fmt.Println()
}
