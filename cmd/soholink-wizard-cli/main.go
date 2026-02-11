package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/wizard"
)

func main() {
	printBanner()

	fmt.Println("Starting SoHoLINK Setup Wizard...")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Ask if ready to start
	fmt.Println("This wizard will:")
	fmt.Println("  • Detect your system hardware")
	fmt.Println("  • Calculate operating costs")
	fmt.Println("  • Suggest competitive pricing")
	fmt.Println("  • Generate complete configuration")
	fmt.Println()
	fmt.Print("Ready to start? (yes/no): ")

	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "yes" && answer != "y" {
		fmt.Println("Setup cancelled.")
		os.Exit(0)
	}

	// Run the full wizard flow
	if err := wizard.DemoWizardFlow(); err != nil {
		fmt.Printf("\n❌ Setup failed: %v\n", err)
		fmt.Println()
		fmt.Print("Press Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Setup completed successfully!")
	fmt.Println()
	fmt.Print("Press Enter to exit...")
	reader.ReadString('\n')
}

func printBanner() {
	banner := `
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║   ███████╗ ██████╗ ██╗  ██╗ ██████╗ ██╗     ██╗███╗   ██╗  ║
║   ██╔════╝██╔═══██╗██║  ██║██╔═══██╗██║     ██║████╗  ██║  ║
║   ███████╗██║   ██║███████║██║   ██║██║     ██║██╔██╗ ██║  ║
║   ╚════██║██║   ██║██╔══██║██║   ██║██║     ██║██║╚██╗██║  ║
║   ███████║╚██████╔╝██║  ██║╚██████╔╝███████╗██║██║ ╚████║  ║
║   ╚══════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝╚═╝  ╚═══╝  ║
║                                                              ║
║          Network Theory Applied Research Institute          ║
║            Federated Cloud Marketplace - Setup Wizard       ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}
