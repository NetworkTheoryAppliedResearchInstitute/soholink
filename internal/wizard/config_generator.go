package wizard

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/did"
)

// ConfigGenerator generates configuration files from wizard inputs.
type ConfigGenerator struct {
	wizardConfig *WizardConfig
	capabilities *SystemCapabilities
	baseDir      string
}

// NewConfigGenerator creates a new configuration generator.
func NewConfigGenerator(wizardCfg *WizardConfig, caps *SystemCapabilities) *ConfigGenerator {
	// Default base directory
	baseDir := getDefaultBaseDir()

	return &ConfigGenerator{
		wizardConfig: wizardCfg,
		capabilities: caps,
		baseDir:      baseDir,
	}
}

// getDefaultBaseDir returns the default SoHoLINK directory.
func getDefaultBaseDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return filepath.Join(homeDir, ".soholink")
}

// SetBaseDir sets a custom base directory.
func (g *ConfigGenerator) SetBaseDir(dir string) {
	g.baseDir = dir
}

// Generate generates all configuration files.
func (g *ConfigGenerator) Generate() error {
	// Create directory structure
	if err := g.createDirectoryStructure(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Generate DID identity
	identityPath, err := g.generateIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate identity: %w", err)
	}
	g.wizardConfig.IdentityPath = identityPath

	// Generate main config.yaml
	configPath, err := g.generateMainConfig()
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}
	g.wizardConfig.ConfigPath = configPath

	// Generate dependency report
	reportPath, err := g.generateDependencyReport()
	if err != nil {
		return fmt.Errorf("failed to generate dependency report: %w", err)
	}
	g.wizardConfig.DependencyReport = reportPath

	return nil
}

// createDirectoryStructure creates the necessary directory structure.
func (g *ConfigGenerator) createDirectoryStructure() error {
	dirs := []string{
		g.baseDir,
		filepath.Join(g.baseDir, "identity"),
		filepath.Join(g.baseDir, "data"),
		filepath.Join(g.baseDir, "logs"),
		filepath.Join(g.baseDir, "vm-storage"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return nil
}

// generateIdentity generates a DID identity and saves keys.
func (g *ConfigGenerator) generateIdentity() (string, error) {
	// Generate Ed25519 keypair
	pub, priv, err := did.GenerateKeypair()
	if err != nil {
		return "", fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Save private key
	privKeyPath := filepath.Join(g.baseDir, "identity", "private.pem")
	if err := did.SavePrivateKey(privKeyPath, priv); err != nil {
		return "", fmt.Errorf("failed to save private key: %w", err)
	}

	// Save public key (for easy access, though it can be derived from private)
	pubKeyPath := filepath.Join(g.baseDir, "identity", "public.pem")
	if err := savePublicKey(pubKeyPath, pub); err != nil {
		return "", fmt.Errorf("failed to save public key: %w", err)
	}

	// Generate DID
	didStr := generateDID(pub)

	// Save DID to file
	didPath := filepath.Join(g.baseDir, "identity", "did.txt")
	if err := os.WriteFile(didPath, []byte(didStr), 0644); err != nil {
		return "", fmt.Errorf("failed to save DID: %w", err)
	}

	return privKeyPath, nil
}

// savePublicKey saves a public key to PEM format.
func savePublicKey(path string, pub ed25519.PublicKey) error {
	// For simplicity, just save as raw bytes
	// In production, would use proper PEM encoding
	return os.WriteFile(path, pub, 0644)
}

// generateDID generates a DID from a public key.
func generateDID(pub ed25519.PublicKey) string {
	// Simple DID format: did:soholink:<base58-encoded-pubkey>
	// In production, would use proper base58 encoding
	// For now, use hex encoding
	return fmt.Sprintf("did:soholink:%x", pub[:16]) // First 16 bytes for brevity
}

// generateMainConfig generates the main config.yaml file.
func (g *ConfigGenerator) generateMainConfig() (string, error) {
	config := make(map[string]interface{})

	// Mode
	config["mode"] = g.wizardConfig.Mode

	// Identity
	privKeyPath := filepath.Join(g.baseDir, "identity", "private.pem")
	pubKeyPath := filepath.Join(g.baseDir, "identity", "public.pem")
	didPath := filepath.Join(g.baseDir, "identity", "did.txt")

	didBytes, err := os.ReadFile(didPath)
	if err != nil {
		return "", err
	}
	didStr := strings.TrimSpace(string(didBytes))

	config["identity"] = map[string]interface{}{
		"did":              didStr,
		"private_key_path": privKeyPath,
		"public_key_path":  pubKeyPath,
	}

	// Resources
	config["resources"] = map[string]interface{}{
		"cpu_cores":              g.wizardConfig.Resources.TotalCPUCores,
		"allocatable_cores":      g.wizardConfig.Resources.AllocatableCores,
		"memory_gb":              g.wizardConfig.Resources.TotalMemoryGB,
		"allocatable_memory_gb":  g.wizardConfig.Resources.AllocatableMemoryGB,
		"storage_gb":             g.wizardConfig.Resources.TotalStorageGB,
		"allocatable_storage_gb": g.wizardConfig.Resources.AllocatableStorageGB,
		"max_vms":                g.wizardConfig.Resources.MaxVMs,
	}

	// Hypervisor
	config["hypervisor"] = map[string]interface{}{
		"type":    g.capabilities.Hypervisor.Type,
		"version": g.capabilities.Hypervisor.Version,
	}

	// Add virtual switch for Windows Hyper-V
	if g.capabilities.OS.Platform == "windows" {
		config["hypervisor"].(map[string]interface{})["virtual_switch"] = "SoHoLINK-vSwitch"
	}

	// Network
	config["network"] = map[string]interface{}{
		"radius_auth_port":    1812,
		"radius_acct_port":    1813,
		"discovery_enabled":   true,
		"discovery_mode":      g.wizardConfig.NetworkMode,
		"firewall_configured": false, // Will be set to true after firewall config
	}

	// Pricing
	config["pricing"] = map[string]interface{}{
		"per_vm_per_hour":       g.wizardConfig.Pricing.PerVMPerHour,
		"currency":              g.wizardConfig.Pricing.Currency,
		"profit_margin_percent": g.wizardConfig.Pricing.ProfitMarginPercent,
	}

	// Cost profile
	config["cost_profile"] = map[string]interface{}{
		"electricity_rate_per_kwh":  g.wizardConfig.CostProfile.ElectricityRatePerKWh,
		"base_power_watts":          g.wizardConfig.CostProfile.BasePowerWatts,
		"load_power_watts":          g.wizardConfig.CostProfile.LoadPowerWatts,
		"cooling_cost_per_hour":     g.wizardConfig.CostProfile.CoolingCostPerHour,
		"hardware_cost":             g.wizardConfig.CostProfile.HardwareCost,
		"hardware_lifespan_years":   g.wizardConfig.CostProfile.HardwareLifespanYears,
		"depreciation_per_hour":     g.wizardConfig.CostProfile.DepreciationPerHour,
		"total_cost_per_hour":       g.wizardConfig.CostProfile.TotalCostPerHour,
	}

	// Policies
	config["policies"] = map[string]interface{}{
		"max_vms_per_customer":      g.wizardConfig.Policies.MaxVMsPerCustomer,
		"max_cpu_cores_per_vm":      g.wizardConfig.Policies.MaxCPUCoresPerVM,
		"max_memory_per_vm_gb":      g.wizardConfig.Policies.MaxMemoryPerVMGB,
		"max_storage_per_vm_gb":     g.wizardConfig.Policies.MaxStoragePerVMGB,
		"min_contract_lead_time":    g.wizardConfig.Policies.MinContractLeadTime,
		"max_contract_duration":     g.wizardConfig.Policies.MaxContractDuration,
		"auto_accept_contracts":     g.wizardConfig.AutoAccept,
		"require_contract_signatures": g.wizardConfig.Policies.RequireSignatures,
	}

	// Security
	config["security"] = map[string]interface{}{
		"rate_limiting_enabled":         g.wizardConfig.Policies.RateLimitingEnabled,
		"rate_limit_requests_per_second": 10,
		"rate_limit_burst_size":         20,
		"log_auth_attempts":             true,
	}

	// Dependencies
	config["dependencies"] = map[string]interface{}{
		"report_path":    filepath.Join(g.baseDir, "dependencies.json"),
		"last_verified":  time.Now().Format(time.RFC3339),
	}

	// Wizard metadata
	config["wizard"] = map[string]interface{}{
		"completed":       true,
		"version":         "1.0.0",
		"completion_time": time.Now().Format(time.RFC3339),
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := []byte("# SoHoLINK Configuration\n# Generated by Deployment Wizard\n# " + time.Now().Format(time.RFC3339) + "\n\n")
	data = append(header, data...)

	// Write to file
	configPath := filepath.Join(g.baseDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, nil
}

// generateDependencyReport generates the dependency report.
func (g *ConfigGenerator) generateDependencyReport() (string, error) {
	tracker := NewDependencyTracker(
		g.capabilities,
		&g.wizardConfig.CostProfile,
		&g.wizardConfig.Pricing,
	)

	report := tracker.GenerateReport()

	// Save JSON report
	jsonPath := filepath.Join(g.baseDir, "dependencies.json")
	if err := report.SaveToFile(jsonPath); err != nil {
		return "", fmt.Errorf("failed to save JSON report: %w", err)
	}

	// Save Markdown report
	mdPath := filepath.Join(g.baseDir, "dependencies.md")
	markdown := report.ExportMarkdown()
	if err := os.WriteFile(mdPath, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("failed to save Markdown report: %w", err)
	}

	// Save HTML report
	htmlPath := filepath.Join(g.baseDir, "dependencies.html")
	html := report.ExportHTML()
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("failed to save HTML report: %w", err)
	}

	return jsonPath, nil
}

// GetWizardConfig returns the wizard configuration.
func (g *ConfigGenerator) GetWizardConfig() *WizardConfig {
	return g.wizardConfig
}

// ExportBackup creates a backup archive of all configuration files.
func (g *ConfigGenerator) ExportBackup(destPath string) error {
	// In production, would create a tar.gz or zip archive
	// For now, just copy the directory

	if err := copyDir(g.baseDir, destPath); err != nil {
		return fmt.Errorf("failed to export backup: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateConfig validates the generated configuration.
func (g *ConfigGenerator) ValidateConfig() error {
	// Check that all required files exist
	requiredFiles := []string{
		filepath.Join(g.baseDir, "config.yaml"),
		filepath.Join(g.baseDir, "identity", "private.pem"),
		filepath.Join(g.baseDir, "identity", "public.pem"),
		filepath.Join(g.baseDir, "identity", "did.txt"),
		filepath.Join(g.baseDir, "dependencies.json"),
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("required file missing: %s", file)
		}
	}

	// Validate DID can be loaded
	didPath := filepath.Join(g.baseDir, "identity", "did.txt")
	didBytes, err := os.ReadFile(didPath)
	if err != nil {
		return fmt.Errorf("failed to read DID: %w", err)
	}

	didStr := strings.TrimSpace(string(didBytes))
	if !strings.HasPrefix(didStr, "did:soholink:") {
		return fmt.Errorf("invalid DID format: %s", didStr)
	}

	// Validate private key can be loaded
	privKeyPath := filepath.Join(g.baseDir, "identity", "private.pem")
	_, err = did.LoadPrivateKey(privKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}

	return nil
}

// GenerateSummary generates a human-readable summary of the configuration.
func (g *ConfigGenerator) GenerateSummary() string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("  SoHoLINK Configuration Summary\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Mode
	sb.WriteString(fmt.Sprintf("Mode: %s\n\n", strings.ToUpper(g.wizardConfig.Mode)))

	// Resources
	sb.WriteString("Resources:\n")
	sb.WriteString(fmt.Sprintf("  • %d VMs (max)\n", g.wizardConfig.Resources.MaxVMs))
	sb.WriteString(fmt.Sprintf("  • %d CPU cores (%d allocatable)\n",
		g.wizardConfig.Resources.TotalCPUCores,
		g.wizardConfig.Resources.AllocatableCores))
	sb.WriteString(fmt.Sprintf("  • %d GB RAM (%d GB allocatable)\n",
		g.wizardConfig.Resources.TotalMemoryGB,
		g.wizardConfig.Resources.AllocatableMemoryGB))
	sb.WriteString(fmt.Sprintf("  • %d GB Storage (%d GB allocatable)\n\n",
		g.wizardConfig.Resources.TotalStorageGB,
		g.wizardConfig.Resources.AllocatableStorageGB))

	// Pricing
	sb.WriteString("Pricing:\n")
	sb.WriteString(fmt.Sprintf("  • $%.3f/hour per VM\n", g.wizardConfig.Pricing.PerVMPerHour))
	sb.WriteString(fmt.Sprintf("  • $%.2f/month per VM\n", g.wizardConfig.Pricing.PerVMPerHour*24*30))
	sb.WriteString(fmt.Sprintf("  • %.0f%% profit margin (%s)\n\n",
		g.wizardConfig.Pricing.ProfitMarginPercent,
		g.wizardConfig.Pricing.PriceMode))

	// Estimated earnings
	monthlyRevenue := g.wizardConfig.Pricing.PerVMPerHour * float64(g.wizardConfig.Resources.MaxVMs) * 24 * 30
	monthlyCost := g.wizardConfig.CostProfile.TotalCostPerHour * 24 * 30
	monthlyProfit := monthlyRevenue - monthlyCost

	sb.WriteString("Estimated Monthly:\n")
	sb.WriteString(fmt.Sprintf("  • Revenue: $%.2f\n", monthlyRevenue))
	sb.WriteString(fmt.Sprintf("  • Costs:   $%.2f\n", monthlyCost))
	sb.WriteString(fmt.Sprintf("  • Profit:  $%.2f\n\n", monthlyProfit))

	// Identity
	didPath := filepath.Join(g.baseDir, "identity", "did.txt")
	if didBytes, err := os.ReadFile(didPath); err == nil {
		didStr := strings.TrimSpace(string(didBytes))
		sb.WriteString(fmt.Sprintf("Identity: %s\n\n", didStr))
	}

	// Files
	sb.WriteString("Configuration Files:\n")
	sb.WriteString(fmt.Sprintf("  • Config:       %s\n", g.wizardConfig.ConfigPath))
	sb.WriteString(fmt.Sprintf("  • Identity:     %s\n", g.wizardConfig.IdentityPath))
	sb.WriteString(fmt.Sprintf("  • Dependencies: %s\n\n", g.wizardConfig.DependencyReport))

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("  Ready to launch! 🚀\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return sb.String()
}

// GetBaseDir returns the base directory.
func (g *ConfigGenerator) GetBaseDir() string {
	return g.baseDir
}
