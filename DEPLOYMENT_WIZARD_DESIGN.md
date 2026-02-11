# Deployment Wizard Design - One-Click Installation

**Date:** 2026-02-10
**Goal:** Enable users to download, install, and configure SoHoLINK with a double-click
**Philosophy:** Gentle guidance through logical steps with intelligent cost calculation

---

## Vision

**Current Reality (Most Software):**
```
User downloads → unzip → read README → edit config files → setup database →
configure networking → calculate pricing manually → get confused → give up
```

**SoHoLINK Experience:**
```
User downloads installer.exe → double-click → wizard guides through setup →
system measures real costs → suggests optimal pricing → documents dependencies →
ready to earn money from compute resources
```

---

## Core Principles

### 1. Zero Technical Barrier
- **No command line** required (optional for power users)
- **No manual config editing** (wizard generates configs)
- **No guessing** (wizard measures and suggests)
- **No undocumented dependencies** (system tracks everything)

### 2. Intelligent Cost Discovery
Users shouldn't calculate hosting costs manually. The system should:
- **Measure actual electricity usage** via hardware monitoring
- **Account for cooling costs** (extra AC for that GPU rack in the living room)
- **Calculate depreciation** (hardware lifespan)
- **Suggest competitive pricing** (what are others charging?)
- **Show profit margins** (revenue vs. expenses)

### 3. Gentle Guidance
The wizard should:
- **Ask one question at a time** (not overwhelming forms)
- **Explain why** each piece of information matters
- **Provide sensible defaults** (most users can just click "Next")
- **Show progress** (step 3 of 7)
- **Allow going back** (fix mistakes easily)

### 4. Dependency Documentation
The system should automatically document:
- **Hardware dependencies** (KVM support, CPU virtualization)
- **Software dependencies** (hypervisor versions, libraries)
- **Configuration state** (what's been configured, what's missing)
- **Health checks** (is everything working?)

---

## Deployment Wizard Flow

### Step 1: Welcome & Mode Selection

```
┌─────────────────────────────────────────────────────────┐
│  Welcome to SoHoLINK!                                   │
│                                                         │
│  Transform your spare compute into income by joining   │
│  the federated cloud marketplace.                      │
│                                                         │
│  What would you like to do?                            │
│                                                         │
│  ○ Rent out my compute resources (Provider)           │
│     Earn money by offering VMs, storage, bandwidth     │
│                                                         │
│  ○ Use federated compute resources (Consumer)         │
│     Run workloads across distributed infrastructure    │
│                                                         │
│  ○ Both (Provider + Consumer)                         │
│     Participate in the full marketplace                │
│                                                         │
│                              [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Why this matters:** Different modes need different configuration

---

### Step 2: System Capabilities Detection

```
┌─────────────────────────────────────────────────────────┐
│  Detecting Your System Capabilities...                  │
│                                                         │
│  ✅ Operating System: Windows 11 Pro                    │
│  ✅ Virtualization: Hyper-V supported and enabled      │
│  ✅ CPU: AMD Ryzen 9 5950X (16 cores, 32 threads)      │
│  ✅ RAM: 64 GB DDR4                                     │
│  ✅ Storage: 2 TB NVMe SSD (1.2 TB free)               │
│  ✅ GPU: NVIDIA RTX 3090 (detected)                    │
│  ✅ Network: 1 Gbps connection                         │
│                                                         │
│  Your system can offer:                                │
│  • Up to 8 concurrent VMs (keeping 8 cores for you)   │
│  • 32 GB allocatable RAM (keeping 32 GB for you)      │
│  • 800 GB VM storage (keeping 400 GB for you)         │
│  • GPU compute (optional, advanced users)              │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Auto-detection includes:**
- CPU cores, architecture, virtualization support
- RAM capacity and current usage
- Storage capacity and filesystem
- GPU presence and compute capability
- Network speed and reliability
- Hypervisor availability (KVM, Hyper-V, etc.)

---

### Step 3: Hardware Cost Measurement

```
┌─────────────────────────────────────────────────────────┐
│  Let's Calculate Your Real Costs                        │
│                                                         │
│  To price competitively, we need to know your actual   │
│  operating costs. This helps you stay profitable!      │
│                                                         │
│  📊 Power Measurement (Automatic)                       │
│  ✅ Current system power draw: 340W                     │
│  ✅ Under load (estimated): 520W                        │
│                                                         │
│  💰 Your Electricity Rate                               │
│  [Enter rate] $0.12 /kWh                               │
│  Where to find this: Check your electricity bill       │
│                                                         │
│  ❄️ Cooling Costs (Optional)                            │
│  Does this system require extra cooling?               │
│  ☑ Yes - I have a GPU rack that heats the room        │
│  [ ] No - Normal computer cooling is fine              │
│                                                         │
│  Extra AC cost: $[0.05]/hour (estimated)               │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Cost calculation engine measures:**
- **Idle power draw** (system monitoring APIs)
- **Load power draw** (stress test or estimation)
- **Cooling overhead** (GPU racks, server closets, etc.)
- **Electricity rate** (user input + public utility data)
- **Hardware depreciation** (estimated lifespan)

**Example calculation:**
```
Base power cost: 520W × $0.12/kWh = $0.062/hour
Cooling overhead: +$0.05/hour
Total power cost: $0.112/hour

Per VM cost: $0.112/hour ÷ 8 VMs = $0.014/hour per VM
Add profit margin (30%): $0.014 × 1.3 = $0.018/hour per VM
Suggested price: $0.02/hour per VM = $14.40/month per VM
```

---

### Step 4: Hardware Depreciation & Lifespan

```
┌─────────────────────────────────────────────────────────┐
│  Hardware Investment & Depreciation                     │
│                                                         │
│  Help us calculate fair pricing by sharing your        │
│  hardware investment. This is optional but recommended. │
│                                                         │
│  💻 Hardware Value (Optional)                           │
│  Approximate total hardware cost:                      │
│  [ ] Don't include depreciation                        │
│  [●] Include depreciation in pricing                   │
│                                                         │
│  Hardware cost: $[3,500]                               │
│  Expected lifespan: [5] years                          │
│                                                         │
│  Calculated depreciation: $0.08/hour                   │
│  (This ensures you recover hardware costs over time)   │
│                                                         │
│  📊 Your Total Operating Cost:                          │
│  • Power: $0.062/hour                                  │
│  • Cooling: $0.050/hour                                │
│  • Depreciation: $0.080/hour                           │
│  • Total: $0.192/hour = $140/month                     │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

---

### Step 5: Market-Based Pricing Suggestions

```
┌─────────────────────────────────────────────────────────┐
│  Suggested Pricing                                      │
│                                                         │
│  Based on your costs and current marketplace rates:    │
│                                                         │
│  📊 Your Costs (per VM instance):                       │
│  Power + Cooling + Depreciation: $0.024/hour           │
│                                                         │
│  💰 Recommended Pricing Tiers:                          │
│                                                         │
│  [●] Competitive (30% profit margin)                   │
│      $0.031/hour = $22.32/month per VM                 │
│      You'll compete well, earn steady income            │
│                                                         │
│  [ ] Premium (50% profit margin)                       │
│      $0.036/hour = $25.92/month per VM                 │
│      Higher earnings, may get fewer bookings            │
│                                                         │
│  [ ] Cost Recovery (10% profit margin)                 │
│      $0.026/hour = $18.72/month per VM                 │
│      Build reputation quickly, minimal profit           │
│                                                         │
│  [ ] Custom pricing: $[____]/hour                      │
│                                                         │
│  🌐 Market Reference (Federated Network Average):       │
│  Similar specs: $0.028 - $0.035/hour                   │
│  AWS Equivalent: $0.096/hour (you're 3x cheaper!)      │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Pricing intelligence:**
- Query federated network for current rates
- Compare to AWS/Azure pricing (show savings)
- Factor in your actual costs
- Suggest profit margins (10%, 30%, 50%)
- Explain tradeoffs (higher price = fewer customers)

---

### Step 6: Identity & Security Setup

```
┌─────────────────────────────────────────────────────────┐
│  Create Your Decentralized Identity (DID)               │
│                                                         │
│  Your DID is your secure identity in the federated     │
│  network. It's cryptographically secured and owned     │
│  by you - not stored on any central server.            │
│                                                         │
│  🔐 Generating Ed25519 Keypair...                       │
│  ✅ Private key created (stored securely on your PC)    │
│  ✅ Public key created                                  │
│                                                         │
│  Your DID:                                             │
│  did:soholink:6H8tqsRpdjkViKRWSUK4oxfF7cZ8p46sWME     │
│                                                         │
│  ⚠️  IMPORTANT: Backup Your Keys                        │
│  Your private key is stored at:                        │
│  C:\Users\YourName\.soholink\identity\private.pem      │
│                                                         │
│  [ ] I have backed up my private key                   │
│                                                         │
│  [Export Backup...]                                    │
│                                                         │
│  💡 Pro Tip: Store your backup in a secure location    │
│     like a password manager or encrypted USB drive.    │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Security features:**
- Auto-generate Ed25519 keypair
- Secure storage (ACL on Windows, 0600 on Unix)
- Backup reminder and export tool
- Clear explanation of DID concept

---

### Step 7: Network Configuration

```
┌─────────────────────────────────────────────────────────┐
│  Network & Firewall Setup                               │
│                                                         │
│  🌐 RADIUS Authentication Server                        │
│  Port 1812 (UDP) - Authentication                      │
│  Port 1813 (UDP) - Accounting                          │
│                                                         │
│  Status: ⚠️  Ports need to be opened                    │
│                                                         │
│  [●] Automatically configure Windows Firewall          │
│  [ ] I'll configure my firewall manually               │
│                                                         │
│  🔧 Auto-configuration will:                            │
│  • Open UDP ports 1812-1813 for RADIUS                 │
│  • Create firewall rules for Hyper-V networking        │
│  • Enable network discovery (federated peers)          │
│                                                         │
│  📡 Federation Discovery                                │
│  How should other nodes find you?                      │
│                                                         │
│  [●] Public (earn maximum income)                      │
│      Your node will be discoverable on the network     │
│                                                         │
│  [ ] Private (trusted partners only)                   │
│      You'll specify which nodes can see your offers    │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Network automation:**
- Detect firewall software
- Auto-create firewall rules (Windows Firewall, iptables)
- Test port accessibility
- Configure NAT traversal if needed
- Set up discovery protocol

---

### Step 8: Dependency Verification & Documentation

```
┌─────────────────────────────────────────────────────────┐
│  System Dependencies Check                              │
│                                                         │
│  Verifying required software and documenting your      │
│  system configuration...                               │
│                                                         │
│  ✅ Hypervisor                                          │
│     Hyper-V 10.0.22000.1 (Windows 11)                  │
│                                                         │
│  ✅ Virtualization                                      │
│     AMD-V enabled in BIOS                              │
│                                                         │
│  ✅ Network Stack                                       │
│     Virtual Switch configured                          │
│                                                         │
│  ✅ Storage                                             │
│     NTFS filesystem, 1.2 TB available                  │
│                                                         │
│  ⚠️  AppArmor Profiles (Linux only)                     │
│     Not applicable on Windows                          │
│                                                         │
│  📋 Dependency Report Generated:                        │
│  C:\Users\YourName\.soholink\dependencies.json         │
│                                                         │
│  This report documents your system configuration and   │
│  will help troubleshoot issues if they arise.          │
│                                                         │
│  [View Full Report]                                    │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

**Dependency documentation includes:**
```json
{
  "timestamp": "2026-02-10T14:30:00Z",
  "platform": {
    "os": "Windows 11 Pro",
    "version": "10.0.22000",
    "architecture": "amd64"
  },
  "hardware": {
    "cpu": {
      "model": "AMD Ryzen 9 5950X",
      "cores": 16,
      "threads": 32,
      "virtualization": "AMD-V",
      "virtualization_enabled": true
    },
    "memory": {
      "total_gb": 64,
      "available_gb": 48
    },
    "storage": {
      "total_gb": 2000,
      "available_gb": 1200,
      "filesystem": "NTFS"
    },
    "gpu": {
      "model": "NVIDIA RTX 3090",
      "compute_capability": "8.6",
      "vram_gb": 24
    }
  },
  "hypervisor": {
    "type": "Hyper-V",
    "version": "10.0.22000.1",
    "features": [
      "nested_virtualization",
      "dynamic_memory",
      "live_migration"
    ]
  },
  "network": {
    "virtual_switch": "SoHoLINK-vSwitch",
    "bandwidth_mbps": 1000,
    "firewall_configured": true,
    "ports_open": [1812, 1813]
  },
  "dependencies": {
    "required": [
      {"name": "Hyper-V", "status": "installed", "version": "10.0.22000.1"},
      {"name": "VirtualSwitch", "status": "configured"}
    ],
    "optional": [
      {"name": "AppArmor", "status": "not_applicable", "platform": "linux_only"}
    ]
  },
  "cost_profile": {
    "electricity_rate_per_kwh": 0.12,
    "base_power_watts": 340,
    "load_power_watts": 520,
    "cooling_cost_per_hour": 0.05,
    "depreciation_per_hour": 0.08,
    "total_cost_per_hour": 0.192
  },
  "pricing": {
    "per_vm_per_hour": 0.031,
    "profit_margin_percent": 30,
    "marketplace_position": "competitive"
  }
}
```

---

### Step 9: Policy & Governance Configuration

```
┌─────────────────────────────────────────────────────────┐
│  Resource Allocation Policies                           │
│                                                         │
│  Set limits to protect your system and maintain        │
│  performance for your own use.                         │
│                                                         │
│  🔒 Resource Limits (per customer)                      │
│                                                         │
│  Max VMs per customer: [4]                             │
│  Max CPU cores per VM: [4]                             │
│  Max RAM per VM: [8] GB                                │
│  Max storage per VM: [100] GB                          │
│                                                         │
│  ⏰ Contract Requirements                               │
│                                                         │
│  Minimum lead time: [24] hours                         │
│  Maximum contract duration: [30] days                  │
│  Auto-accept contracts: [●] Yes  [ ] No                │
│                                                         │
│  💡 Auto-accept means:                                  │
│  Contracts matching your policies are automatically    │
│  accepted. You can review and cancel anytime.          │
│                                                         │
│  🛡️  Security Policies                                  │
│                                                         │
│  [●] Require contract signatures (recommended)         │
│  [●] Enable rate limiting (10 req/sec per IP)         │
│  [●] Log all authentication attempts                   │
│                                                         │
│                    [< Back]  [Next >]                   │
└─────────────────────────────────────────────────────────┘
```

---

### Step 10: Final Review & Launch

```
┌─────────────────────────────────────────────────────────┐
│  Ready to Launch!                                       │
│                                                         │
│  📊 Configuration Summary:                              │
│                                                         │
│  Mode: Provider (rent out resources)                   │
│  Resources: 8 VMs × 4 cores × 8 GB RAM                │
│  Pricing: $0.031/hour ($22.32/month per VM)           │
│  Profit: 30% above costs ($56/month estimated)         │
│                                                         │
│  Identity: did:soholink:6H8tqsRp...                    │
│  Network: Public discovery enabled                     │
│  Firewall: Configured automatically                    │
│                                                         │
│  Dependencies: All requirements met ✅                  │
│  Security: Rate limiting enabled ✅                     │
│  Policies: Auto-accept enabled ✅                       │
│                                                         │
│  📁 Configuration saved to:                             │
│  C:\Users\YourName\.soholink\config.yaml               │
│                                                         │
│  📋 Dependency report saved to:                         │
│  C:\Users\YourName\.soholink\dependencies.json         │
│                                                         │
│  [View Configuration]  [Export Backup]                 │
│                                                         │
│                    [< Back]  [Finish & Launch]          │
└─────────────────────────────────────────────────────────┘
```

---

## Technical Implementation

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 Deployment Wizard                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │   UI Layer   │  │  Business    │  │  System     │  │
│  │   (Fyne)     │→ │  Logic       │→ │  Detection  │  │
│  └──────────────┘  └──────────────┘  └─────────────┘  │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │   Cost       │  │  Config      │  │  Dependency │  │
│  │   Calculator │  │  Generator   │  │  Tracker    │  │
│  └──────────────┘  └──────────────┘  └─────────────┘  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Components to Build

#### 1. System Detection Module
**File:** `internal/wizard/detection.go`

```go
package wizard

type SystemCapabilities struct {
    OS           OSInfo
    CPU          CPUInfo
    Memory       MemoryInfo
    Storage      StorageInfo
    GPU          *GPUInfo      // Optional
    Hypervisor   HypervisorInfo
    Network      NetworkInfo
    Virtualization bool
}

func DetectSystemCapabilities() (*SystemCapabilities, error)
func (s *SystemCapabilities) CalculateAvailableResources() *ResourceAllocation
func (s *SystemCapabilities) ValidateProviderCapability() error
```

#### 2. Cost Calculator Module
**File:** `internal/wizard/cost_calculator.go`

```go
package wizard

type CostProfile struct {
    ElectricityRatePerKWh float64
    BasePowerWatts        float64
    LoadPowerWatts        float64
    CoolingCostPerHour    float64
    HardwareCost          float64
    HardwareLifespanYears float64
}

type CostCalculator struct {
    profile *CostProfile
}

func (c *CostCalculator) CalculateHourlyCost() float64
func (c *CostCalculator) SuggestPricing(profitMargin float64) float64
func (c *CostCalculator) CompareToMarket() *MarketComparison
```

**Power measurement:**
```go
// Windows: Use Performance Counters
func measureWindowsPowerDraw() (float64, error) {
    // Query \Processor(_Total)\% Processor Time
    // Query \Memory\Available MBytes
    // Estimate from CPU/RAM usage + base load
}

// Linux: Use /sys/class/powercap or sensors
func measureLinuxPowerDraw() (float64, error) {
    // Read /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj
    // Or use `sensors` command output
}

// macOS: Use IOKit
func measureMacOSPowerDraw() (float64, error) {
    // Use IOKit to query power metrics
}
```

#### 3. Dependency Documentation Module
**File:** `internal/wizard/dependencies.go`

```go
package wizard

type DependencyReport struct {
    Timestamp      time.Time
    Platform       PlatformInfo
    Hardware       HardwareInfo
    Hypervisor     HypervisorInfo
    Network        NetworkInfo
    Dependencies   []Dependency
    CostProfile    CostProfile
    Pricing        PricingConfig
}

func GenerateDependencyReport(sys *SystemCapabilities) *DependencyReport
func (d *DependencyReport) SaveToFile(path string) error
func (d *DependencyReport) ExportMarkdown() string
func (d *DependencyReport) ExportJSON() ([]byte, error)
```

#### 4. Configuration Generator
**File:** `internal/wizard/config_generator.go`

```go
package wizard

type WizardConfig struct {
    Mode              string  // "provider", "consumer", "both"
    Resources         ResourceAllocation
    Pricing           PricingConfig
    Identity          IdentityConfig
    Network           NetworkConfig
    Policies          PolicyConfig
    DependencyReport  string  // Path to report
}

func (w *WizardConfig) GenerateYAML() ([]byte, error)
func (w *WizardConfig) WriteConfigFiles() error
func (w *WizardConfig) ValidateConfig() error
```

#### 5. Firewall Configuration
**File:** `internal/wizard/firewall.go`

```go
package wizard

type FirewallConfigurator interface {
    OpenPorts(ports []int, protocol string) error
    CreateRule(name string, rule FirewallRule) error
    TestPortAccessibility(port int) error
}

// Windows implementation
type WindowsFirewall struct{}
func (w *WindowsFirewall) OpenPorts(ports []int, protocol string) error {
    // Use netsh advfirewall firewall add rule
}

// Linux implementation
type LinuxFirewall struct{}
func (l *LinuxFirewall) OpenPorts(ports []int, protocol string) error {
    // Use iptables or firewalld
}
```

#### 6. Wizard UI
**File:** `ui/wizard/wizard.go`

```go
package wizard

import "fyne.io/fyne/v2"

type DeploymentWizard struct {
    window    fyne.Window
    steps     []WizardStep
    currentStep int
    config    *WizardConfig
}

type WizardStep interface {
    Title() string
    Render() fyne.CanvasObject
    Validate() error
    OnNext() error
    OnBack() error
}

func NewDeploymentWizard() *DeploymentWizard
func (w *DeploymentWizard) Run()
```

---

## Installation Packaging

### Windows

**Installer:** `SoHoLINK-Setup.exe` (NSIS or WiX)

```
SoHoLINK-Setup.exe includes:
- soholink.exe (main binary)
- bundled dependencies (if any)
- default configuration templates
- documentation (HTML)
- uninstaller

Installation process:
1. User double-clicks SoHoLINK-Setup.exe
2. UAC prompt (requires admin for first-time setup)
3. Install to C:\Program Files\SoHoLINK
4. Create Start Menu shortcuts
5. Launch deployment wizard automatically
6. Wizard guides through configuration
7. System starts on completion
```

### macOS

**Installer:** `SoHoLINK.dmg` or `SoHoLINK.pkg`

```
SoHoLINK.dmg includes:
- SoHoLINK.app (application bundle)
- Documentation folder
- Uninstall script

Installation process:
1. User double-clicks SoHoLINK.dmg
2. Drag SoHoLINK.app to Applications
3. First launch opens deployment wizard
4. Wizard requests necessary permissions
5. Configuration completes
6. System starts
```

### Linux

**Package:** `soholink_1.0.0_amd64.deb` (Debian/Ubuntu)
**Package:** `soholink-1.0.0-1.x86_64.rpm` (RHEL/Fedora)

```
Installation process:
sudo dpkg -i soholink_1.0.0_amd64.deb
# or
sudo rpm -i soholink-1.0.0-1.x86_64.rpm

Post-install:
- soholink-wizard (command-line wizard)
- or launch GUI: soholink-ui
```

---

## Intelligent Features

### 1. Power Monitoring Integration

**Windows:**
```go
// Use Windows Performance Counters
import "github.com/StackExchange/wmi"

type Win32_PerfFormattedData_Counters_ProcessorInformation struct {
    PercentProcessorTime uint64
}

func GetCPUUsage() (uint64, error) {
    var dst []Win32_PerfFormattedData_Counters_ProcessorInformation
    query := "SELECT PercentProcessorTime FROM Win32_PerfFormattedData_Counters_ProcessorInformation WHERE Name='_Total'"
    err := wmi.Query(query, &dst)
    if err != nil || len(dst) == 0 {
        return 0, err
    }
    return dst[0].PercentProcessorTime, nil
}

// Estimate power from usage
func EstimatePowerDraw(cpuPercent, memoryPercent float64, hasGPU bool) float64 {
    basePower := 80.0  // watts (idle motherboard, fans, drives)
    cpuPower := (cpuPercent / 100.0) * 125.0  // typical desktop CPU TDP
    memoryPower := (memoryPercent / 100.0) * 30.0  // RAM power
    gpuPower := 0.0
    if hasGPU {
        gpuPower = 250.0  // typical GPU power (adjust based on model)
    }
    return basePower + cpuPower + memoryPower + gpuPower
}
```

**Linux:**
```go
import "github.com/shirou/gopsutil/v3/cpu"

func MeasureLinuxPower() (float64, error) {
    // Try RAPL first (Intel Running Average Power Limit)
    power, err := readRAPL()
    if err == nil {
        return power, nil
    }

    // Fall back to estimation from CPU usage
    percent, err := cpu.Percent(time.Second, false)
    if err != nil {
        return 0, err
    }
    return EstimatePowerDraw(percent[0], 0, false), nil
}

func readRAPL() (float64, error) {
    // Read /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj
    data, err := os.ReadFile("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj")
    if err != nil {
        return 0, err
    }
    // Parse and calculate watts
    // ...
}
```

### 2. Cooling Cost Detection

```go
type CoolingProfile struct {
    HasDedicatedCooling bool
    RoomSize           float64  // square meters
    GPUCount           int
    AmbientTemp        float64  // celsius
}

func EstimateCoolingCost(profile *CoolingProfile, electricityRate float64) float64 {
    if !profile.HasDedicatedCooling {
        return 0.0  // No extra cooling
    }

    // Estimate BTU needed to cool heat from GPUs
    heatBTU := float64(profile.GPUCount) * 853.0  // 250W GPU = ~853 BTU/hr

    // AC efficiency (typical window AC: 10 BTU/watt)
    acWatts := heatBTU / 10.0

    // Cost per hour
    costPerHour := (acWatts / 1000.0) * electricityRate

    return costPerHour
}
```

### 3. Market Price Discovery

```go
type MarketPriceDiscovery struct {
    federationClient *federation.Client
}

func (m *MarketPriceDiscovery) GetCurrentRates(specs *ResourceSpecs) (*MarketRates, error) {
    // Query federated network for similar offerings
    offers, err := m.federationClient.QueryOffers(specs)
    if err != nil {
        return nil, err
    }

    // Calculate percentiles
    prices := extractPrices(offers)
    sort.Float64s(prices)

    return &MarketRates{
        Min:       prices[0],
        P25:       percentile(prices, 0.25),
        Median:    percentile(prices, 0.50),
        P75:       percentile(prices, 0.75),
        Max:       prices[len(prices)-1],
        Count:     len(prices),
        Timestamp: time.Now(),
    }, nil
}

func (m *MarketPriceDiscovery) CompareToAWS(specs *ResourceSpecs) *AWSComparison {
    // Look up equivalent AWS instance type
    awsPrice := lookupAWSPrice(specs)

    return &AWSComparison{
        AWSPrice:      awsPrice,
        SavingsPercent: ((awsPrice - yourPrice) / awsPrice) * 100,
    }
}
```

---

## Example Configuration Output

**Generated:** `~/.soholink/config.yaml`

```yaml
# SoHoLINK Configuration
# Generated by Deployment Wizard on 2026-02-10 14:30:00

mode: provider

identity:
  did: "did:soholink:6H8tqsRpdjkViKRWSUK4oxfF7cZ8p46sWME"
  private_key_path: "~/.soholink/identity/private.pem"
  public_key_path: "~/.soholink/identity/public.pem"

resources:
  cpu_cores: 16
  allocatable_cores: 8  # Keep 8 for host system
  memory_gb: 64
  allocatable_memory_gb: 32
  storage_gb: 2000
  allocatable_storage_gb: 800
  max_vms: 8

hypervisor:
  type: "hyper-v"
  version: "10.0.22000.1"
  virtual_switch: "SoHoLINK-vSwitch"

network:
  radius_auth_port: 1812
  radius_acct_port: 1813
  discovery_enabled: true
  discovery_mode: "public"
  firewall_configured: true

pricing:
  per_vm_per_hour: 0.031
  currency: "USD"
  profit_margin_percent: 30

cost_profile:
  electricity_rate_per_kwh: 0.12
  base_power_watts: 340
  load_power_watts: 520
  cooling_cost_per_hour: 0.05
  hardware_cost: 3500
  hardware_lifespan_years: 5
  depreciation_per_hour: 0.08
  total_cost_per_hour: 0.192

policies:
  max_vms_per_customer: 4
  max_cpu_cores_per_vm: 4
  max_memory_per_vm_gb: 8
  max_storage_per_vm_gb: 100
  min_contract_lead_time: "24h"
  max_contract_duration: "720h"  # 30 days
  auto_accept_contracts: true
  require_contract_signatures: true

security:
  rate_limiting_enabled: true
  rate_limit_requests_per_second: 10
  rate_limit_burst_size: 20
  log_auth_attempts: true

dependencies:
  report_path: "~/.soholink/dependencies.json"
  last_verified: "2026-02-10T14:30:00Z"

# Wizard metadata
wizard:
  completed: true
  version: "1.0.0"
  completion_time: "2026-02-10T14:30:00Z"
```

---

## Success Metrics

### User Experience Goals

1. **Time to First Revenue**
   - Target: < 15 minutes from download to accepting first contract
   - Measured: wizard completion time + first contract acceptance

2. **Configuration Accuracy**
   - Target: 95% of users complete wizard without errors
   - Measured: wizard completion rate vs. abandonment rate

3. **Pricing Competitiveness**
   - Target: 80% of wizard-generated prices within market range
   - Measured: compare wizard prices to actual market rates

4. **Dependency Documentation**
   - Target: 100% of system dependencies documented automatically
   - Measured: manual vs. auto-detected dependencies

### Technical Goals

1. **Cross-Platform Support**
   - Windows 10/11 (Hyper-V)
   - Linux (KVM)
   - macOS (future: Virtualization.framework)

2. **Installation Success Rate**
   - Target: 98% successful installations
   - Measured: telemetry (opt-in)

3. **Auto-Configuration Success**
   - Target: 90% of firewall rules configured automatically
   - Measured: manual intervention rate

---

## Next Steps - Implementation Order

### Phase 1: Core Detection (Week 1)
1. System capabilities detection
2. Hardware inventory
3. Hypervisor detection
4. Dependency reporting

### Phase 2: Cost Calculator (Week 1-2)
1. Power measurement integration
2. Cooling cost estimation
3. Depreciation calculator
4. Market price discovery

### Phase 3: Wizard UI (Week 2-3)
1. Fyne-based wizard interface
2. Step-by-step flow
3. Validation and error handling
4. Configuration generation

### Phase 4: Auto-Configuration (Week 3-4)
1. Firewall configuration
2. Identity generation
3. Network setup
4. Service installation

### Phase 5: Packaging (Week 4)
1. Windows installer (NSIS/WiX)
2. macOS bundle (.dmg/.pkg)
3. Linux packages (.deb/.rpm)
4. Documentation

---

**Ready to transform deployment from complex to magical!** ✨
