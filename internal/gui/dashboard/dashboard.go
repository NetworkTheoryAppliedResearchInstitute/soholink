//go:build gui

// Package dashboard provides operator-facing GUI views for SoHoLINK
// built with the Fyne toolkit. It includes:
//   - A multi-step SetupWizard for first-time node configuration and install.
//   - A tabbed Dashboard for real-time node status, user management, accounting
//     log viewing, LBTAS reputation scores, and managed-service health.
//
// NOTE: The Fyne GUI toolkit (fyne.io/fyne/v2) is an optional dependency
// that is only required when building with the "gui" build tag.
package dashboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fyneApp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/app"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// ---------------------------------------------------------------------------
// Setup wizard types
// ---------------------------------------------------------------------------

// wizardStep enumerates the pages of the setup wizard.
type wizardStep int

const (
	stepWelcome wizardStep = iota
	stepLicense
	stepDeploymentMode
	stepConfiguration
	stepAdvancedConfig
	stepReview
	stepInstallProgress
	stepComplete
)

// wizardState carries mutable form values across wizard steps.
type wizardState struct {
	// License acceptance
	LicenseAccepted bool

	// Deployment mode
	DeploymentMode string // "standalone" or "saas"

	// Basic configuration
	NodeName  string
	AuthPort  string
	AcctPort  string
	DataDir   string
	Secret    string

	// Advanced configuration
	P2PEnabled       bool
	P2PPort          string
	UpdatesEnabled   bool
	MetricsEnabled   bool
	MetricsPort      string
	PaymentsEnabled  bool
	StorageLimitGB   int

	LogOutput []string
}

// RunSetupWizard creates and displays a multi-step installer wizard.  It
// collects the node name, RADIUS ports, data directory, and shared secret
// and then drives config.EnsureDirectories + store initialisation with a
// live progress bar.
func RunSetupWizard(cfg *config.Config, s *store.Store) {
	a := fyneApp.New()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("SoHoLINK Setup Wizard")
	w.Resize(fyne.NewSize(700, 520))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	state := &wizardState{
		LicenseAccepted: false,
		DeploymentMode:  "standalone",
		NodeName:        cfg.Node.Name,
		AuthPort:        portFromAddr(cfg.Radius.AuthAddress, "1812"),
		AcctPort:        portFromAddr(cfg.Radius.AcctAddress, "1813"),
		DataDir:         cfg.Storage.BasePath,
		Secret:          cfg.Radius.SharedSecret,
		P2PEnabled:      true,
		P2PPort:         "9090",
		UpdatesEnabled:  true,
		MetricsEnabled:  true,
		MetricsPort:     "9100",
		PaymentsEnabled: false,
		StorageLimitGB:  100,
	}

	// Forward-declare so steps can reference each other.
	var showStep func(step wizardStep)
	showStep = func(step wizardStep) {
		switch step {
		case stepWelcome:
			w.SetContent(buildWelcomePage(func() { showStep(stepLicense) }))
		case stepLicense:
			w.SetContent(buildLicensePage(state, func() { showStep(stepWelcome) }, func() { showStep(stepDeploymentMode) }))
		case stepDeploymentMode:
			w.SetContent(buildDeploymentModePage(state, func() { showStep(stepLicense) }, func() { showStep(stepConfiguration) }))
		case stepConfiguration:
			w.SetContent(buildConfigPage(state, func() { showStep(stepDeploymentMode) }, func() { showStep(stepAdvancedConfig) }))
		case stepAdvancedConfig:
			w.SetContent(buildAdvancedConfigPage(state, func() { showStep(stepConfiguration) }, func() { showStep(stepReview) }))
		case stepReview:
			w.SetContent(buildReviewPage(state, func() { showStep(stepAdvancedConfig) }, func() { showStep(stepInstallProgress) }))
		case stepInstallProgress:
			w.SetContent(buildInstallPage(state, cfg, s, w, func() { showStep(stepComplete) }))
		case stepComplete:
			w.SetContent(buildCompletePage(state, w))
		}
	}

	showStep(stepWelcome)
	w.ShowAndRun()
}

// ---------------------------------------------------------------------------
// Wizard step builders
// ---------------------------------------------------------------------------

func buildWelcomePage(onNext func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Welcome to SoHoLINK",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	subtitle := widget.NewLabelWithStyle(
		"Decentralised AAA Node Installer",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	body := widget.NewLabel(
		"This wizard will guide you through the initial configuration of your " +
			"SoHoLINK node. You will be asked to set a node name, choose RADIUS " +
			"ports, select a data directory, and review your settings before " +
			"installation begins.\n\n" +
			"Press Next to continue.")
	body.Wrapping = fyne.TextWrapWord

	next := widget.NewButton("Next", onNext)
	next.Importance = widget.HighImportance

	spacer := layout.NewSpacer()
	return container.NewBorder(
		container.NewVBox(title, subtitle, widget.NewSeparator()),
		container.NewHBox(spacer, next),
		nil, nil,
		container.NewPadded(body),
	)
}

func buildLicensePage(state *wizardState, onBack, onNext func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"License Agreement",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// License text (placeholder until GAP 1 is complete)
	licenseText := `FedAAA - License Agreement

Copyright (c) 2024 Network Theory Applied Research Institute

[License text to be determined - see GAP 1]

This software is provided for evaluation and testing purposes.
The final license will be determined by the project maintainers.

By accepting this agreement, you acknowledge that:
1. You are using pre-release software
2. The license terms may change before final release
3. You agree to use this software in compliance with applicable laws
4. No warranty is provided for this software

For the latest license information, visit:
https://github.com/NetworkTheoryAppliedResearchInstitute/soholink`

	licenseBox := widget.NewMultiLineEntry()
	licenseBox.SetText(licenseText)
	licenseBox.Wrapping = fyne.TextWrapWord
	licenseBox.Disable()

	acceptCheck := widget.NewCheck("I accept the terms of the license agreement", func(checked bool) {
		state.LicenseAccepted = checked
	})
	acceptCheck.SetChecked(state.LicenseAccepted)

	back := widget.NewButton("Back", onBack)
	next := widget.NewButton("Next", func() {
		if !state.LicenseAccepted {
			dialog.ShowInformation("License Required", "You must accept the license agreement to continue.", nil)
			return
		}
		onNext()
	})
	next.Importance = widget.HighImportance

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewVBox(acceptCheck, widget.NewSeparator(), container.NewHBox(back, layout.NewSpacer(), next)),
		nil, nil,
		container.NewPadded(licenseBox),
	)
}

func buildDeploymentModePage(state *wizardState, onBack, onNext func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Deployment Mode",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	subtitle := widget.NewLabel("Choose how you want to deploy FedAAA:")
	subtitle.Wrapping = fyne.TextWrapWord

	// Standalone mode option
	standaloneTitle := widget.NewLabelWithStyle("Standalone Node", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	standaloneDesc := widget.NewLabel(
		"Run FedAAA as an independent node on your own infrastructure. " +
			"You have full control over the node, its data, and its operations. " +
			"Recommended for self-hosted deployments, testing, and development.")
	standaloneDesc.Wrapping = fyne.TextWrapWord

	standaloneCard := widget.NewCard("", "", container.NewVBox(
		standaloneTitle,
		standaloneDesc,
	))

	// SaaS mode option
	saasTitle := widget.NewLabelWithStyle("SaaS / Managed Mode", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	saasDesc := widget.NewLabel(
		"Connect to a managed FedAAA service provider. " +
			"Reduced operational overhead with centralized management and support. " +
			"Recommended for production deployments where uptime and support are critical.")
	saasDesc.Wrapping = fyne.TextWrapWord

	saasCard := widget.NewCard("", "", container.NewVBox(
		saasTitle,
		saasDesc,
	))

	// Mode selection radio
	modeRadio := widget.NewRadioGroup([]string{"Standalone Node", "SaaS / Managed Mode"}, func(selected string) {
		if selected == "Standalone Node" {
			state.DeploymentMode = "standalone"
		} else {
			state.DeploymentMode = "saas"
		}
	})

	if state.DeploymentMode == "standalone" {
		modeRadio.SetSelected("Standalone Node")
	} else {
		modeRadio.SetSelected("SaaS / Managed Mode")
	}

	back := widget.NewButton("Back", onBack)
	next := widget.NewButton("Next", onNext)
	next.Importance = widget.HighImportance

	content := container.NewVBox(
		subtitle,
		widget.NewSeparator(),
		standaloneCard,
		saasCard,
		widget.NewSeparator(),
		modeRadio,
	)

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewHBox(back, layout.NewSpacer(), next),
		nil, nil,
		container.NewPadded(content),
	)
}

func buildConfigPage(state *wizardState, onBack, onNext func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Configuration",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("my-soholink-node")
	nameEntry.SetText(state.NodeName)
	nameEntry.OnChanged = func(v string) { state.NodeName = v }

	authEntry := widget.NewEntry()
	authEntry.SetPlaceHolder("1812")
	authEntry.SetText(state.AuthPort)
	authEntry.OnChanged = func(v string) { state.AuthPort = v }

	acctEntry := widget.NewEntry()
	acctEntry.SetPlaceHolder("1813")
	acctEntry.SetText(state.AcctPort)
	acctEntry.OnChanged = func(v string) { state.AcctPort = v }

	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder(config.DefaultDataDir())
	dirEntry.SetText(state.DataDir)
	dirEntry.OnChanged = func(v string) { state.DataDir = v }

	secretEntry := widget.NewPasswordEntry()
	secretEntry.SetPlaceHolder("shared secret")
	secretEntry.SetText(state.Secret)
	secretEntry.OnChanged = func(v string) { state.Secret = v }

	form := widget.NewForm(
		widget.NewFormItem("Node Name", nameEntry),
		widget.NewFormItem("Auth Port", authEntry),
		widget.NewFormItem("Acct Port", acctEntry),
		widget.NewFormItem("Data Dir", dirEntry),
		widget.NewFormItem("Shared Secret", secretEntry),
	)

	back := widget.NewButton("Back", onBack)
	next := widget.NewButton("Next", onNext)
	next.Importance = widget.HighImportance

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewHBox(back, layout.NewSpacer(), next),
		nil, nil,
		container.NewPadded(form),
	)
}

func buildAdvancedConfigPage(state *wizardState, onBack, onNext func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Advanced Configuration",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	subtitle := widget.NewLabel("Configure optional features (recommended defaults are pre-selected):")
	subtitle.Wrapping = fyne.TextWrapWord

	// P2P Networking
	p2pCheck := widget.NewCheck("Enable P2P mesh networking", func(checked bool) {
		state.P2PEnabled = checked
	})
	p2pCheck.SetChecked(state.P2PEnabled)

	p2pPortEntry := widget.NewEntry()
	p2pPortEntry.SetPlaceHolder("9090")
	p2pPortEntry.SetText(state.P2PPort)
	p2pPortEntry.OnChanged = func(v string) { state.P2PPort = v }

	p2pCard := widget.NewCard("P2P Mesh Networking", "", container.NewVBox(
		p2pCheck,
		widget.NewLabel("Discover and connect to peers via mDNS multicast"),
		widget.NewForm(widget.NewFormItem("P2P Port", p2pPortEntry)),
	))

	// Auto-updates
	updatesCheck := widget.NewCheck("Enable automatic updates", func(checked bool) {
		state.UpdatesEnabled = checked
	})
	updatesCheck.SetChecked(state.UpdatesEnabled)

	updatesCard := widget.NewCard("Auto-Updates", "", container.NewVBox(
		updatesCheck,
		widget.NewLabel("Automatically check for and install security updates"),
	))

	// Metrics and monitoring
	metricsCheck := widget.NewCheck("Enable Prometheus metrics", func(checked bool) {
		state.MetricsEnabled = checked
	})
	metricsCheck.SetChecked(state.MetricsEnabled)

	metricsPortEntry := widget.NewEntry()
	metricsPortEntry.SetPlaceHolder("9100")
	metricsPortEntry.SetText(state.MetricsPort)
	metricsPortEntry.OnChanged = func(v string) { state.MetricsPort = v }

	metricsCard := widget.NewCard("Monitoring", "", container.NewVBox(
		metricsCheck,
		widget.NewLabel("Export metrics for Prometheus/Grafana monitoring"),
		widget.NewForm(widget.NewFormItem("Metrics Port", metricsPortEntry)),
	))

	// Payment processing
	paymentsCheck := widget.NewCheck("Enable payment processors", func(checked bool) {
		state.PaymentsEnabled = checked
	})
	paymentsCheck.SetChecked(state.PaymentsEnabled)

	paymentsCard := widget.NewCard("Payments", "", container.NewVBox(
		paymentsCheck,
		widget.NewLabel("Enable Stripe, Lightning, and other payment methods"),
		widget.NewLabel("(Requires additional configuration after installation)"),
	))

	// Storage limit
	storageLimitEntry := widget.NewEntry()
	storageLimitEntry.SetPlaceHolder("100")
	storageLimitEntry.SetText(fmt.Sprintf("%d", state.StorageLimitGB))
	storageLimitEntry.OnChanged = func(v string) {
		var limit int
		fmt.Sscanf(v, "%d", &limit)
		if limit > 0 {
			state.StorageLimitGB = limit
		}
	}

	storageCard := widget.NewCard("Storage", "", container.NewVBox(
		widget.NewLabel("Maximum storage allocation for workloads and data"),
		widget.NewForm(widget.NewFormItem("Storage Limit (GB)", storageLimitEntry)),
	))

	back := widget.NewButton("Back", onBack)
	next := widget.NewButton("Next", onNext)
	next.Importance = widget.HighImportance

	content := container.NewVScroll(container.NewVBox(
		subtitle,
		widget.NewSeparator(),
		p2pCard,
		updatesCard,
		metricsCard,
		paymentsCard,
		storageCard,
	))

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewHBox(back, layout.NewSpacer(), next),
		nil, nil,
		container.NewPadded(content),
	)
}

func buildReviewPage(state *wizardState, onBack, onInstall func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Review Settings",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Basic configuration summary
	basicSummary := widget.NewLabel(fmt.Sprintf(
		"Deployment Mode: %s\n"+
			"Node Name:       %s\n"+
			"Auth Port:       %s\n"+
			"Acct Port:       %s\n"+
			"Data Dir:        %s\n"+
			"Shared Secret:   %s",
		state.DeploymentMode,
		state.NodeName,
		state.AuthPort,
		state.AcctPort,
		state.DataDir,
		maskSecret(state.Secret),
	))
	basicSummary.TextStyle = fyne.TextStyle{Monospace: true}

	basicCard := widget.NewCard("Basic Configuration", "", basicSummary)

	// Advanced configuration summary
	p2pStatus := "Disabled"
	if state.P2PEnabled {
		p2pStatus = fmt.Sprintf("Enabled (port %s)", state.P2PPort)
	}

	updatesStatus := "Disabled"
	if state.UpdatesEnabled {
		updatesStatus = "Enabled"
	}

	metricsStatus := "Disabled"
	if state.MetricsEnabled {
		metricsStatus = fmt.Sprintf("Enabled (port %s)", state.MetricsPort)
	}

	paymentsStatus := "Disabled"
	if state.PaymentsEnabled {
		paymentsStatus = "Enabled"
	}

	advancedSummary := widget.NewLabel(fmt.Sprintf(
		"P2P Networking:     %s\n"+
			"Auto-Updates:       %s\n"+
			"Metrics:            %s\n"+
			"Payments:           %s\n"+
			"Storage Limit:      %d GB",
		p2pStatus,
		updatesStatus,
		metricsStatus,
		paymentsStatus,
		state.StorageLimitGB,
	))
	advancedSummary.TextStyle = fyne.TextStyle{Monospace: true}

	advancedCard := widget.NewCard("Advanced Configuration", "", advancedSummary)

	body := widget.NewLabel("Please confirm the settings above. Press Install to begin " +
		"creating directories and initialising the database.")
	body.Wrapping = fyne.TextWrapWord

	back := widget.NewButton("Back", onBack)
	install := widget.NewButton("Install", onInstall)
	install.Importance = widget.HighImportance

	content := container.NewVScroll(container.NewVBox(
		basicCard,
		advancedCard,
		widget.NewSeparator(),
		body,
	))

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewHBox(back, layout.NewSpacer(), install),
		nil, nil,
		container.NewPadded(content),
	)
}

func buildInstallPage(state *wizardState, cfg *config.Config, s *store.Store, w fyne.Window, onDone func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Installing...",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	progress := widget.NewProgressBar()
	logBox := widget.NewMultiLineEntry()
	logBox.Wrapping = fyne.TextWrapWord
	logBox.Disable()

	appendLog := func(msg string) {
		state.LogOutput = append(state.LogOutput, msg)
		logBox.SetText(strings.Join(state.LogOutput, "\n"))
	}

	// Run the installation steps asynchronously so the UI stays responsive.
	go func() {
		steps := []struct {
			label string
			fn    func() error
		}{
			{"Applying configuration...", func() error {
				cfg.Node.Name = state.NodeName
				cfg.Radius.AuthAddress = fmt.Sprintf("0.0.0.0:%s", state.AuthPort)
				cfg.Radius.AcctAddress = fmt.Sprintf("0.0.0.0:%s", state.AcctPort)
				cfg.Storage.BasePath = state.DataDir
				cfg.Radius.SharedSecret = state.Secret
				return nil
			}},
			{"Creating directories...", func() error {
				return config.EnsureDirectories(cfg)
			}},
			{"Initialising database...", func() error {
				dbPath := cfg.DatabasePath()
				_, err := store.NewStore(dbPath)
				return err
			}},
			{"Writing node info...", func() error {
				if s != nil {
					ctx := context.Background()
					_ = s.SetNodeInfo(ctx, "node_name", state.NodeName)
					_ = s.SetNodeInfo(ctx, "deployment_mode", state.DeploymentMode)
					_ = s.SetNodeInfo(ctx, "installed_at", time.Now().Format(time.RFC3339))
					_ = s.SetNodeInfo(ctx, "platform", runtime.GOOS+"/"+runtime.GOARCH)
					_ = s.SetNodeInfo(ctx, "p2p_enabled", fmt.Sprintf("%t", state.P2PEnabled))
					_ = s.SetNodeInfo(ctx, "updates_enabled", fmt.Sprintf("%t", state.UpdatesEnabled))
					_ = s.SetNodeInfo(ctx, "metrics_enabled", fmt.Sprintf("%t", state.MetricsEnabled))
					_ = s.SetNodeInfo(ctx, "payments_enabled", fmt.Sprintf("%t", state.PaymentsEnabled))
				}
				return nil
			}},
			{"Verifying installation...", func() error {
				// Quick sanity check: data dir must exist.
				if _, err := os.Stat(state.DataDir); err != nil {
					return fmt.Errorf("data directory missing: %w", err)
				}
				return nil
			}},
		}

		for i, step := range steps {
			appendLog(step.label)
			if err := step.fn(); err != nil {
				appendLog(fmt.Sprintf("ERROR: %v", err))
				dialog.ShowError(err, w)
				return
			}
			appendLog("  done.")
			progress.SetValue(float64(i+1) / float64(len(steps)))
			time.Sleep(200 * time.Millisecond) // brief pause so the user can read the log
		}

		appendLog("\nInstallation complete!")
		onDone()
	}()

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator(), progress),
		nil, nil, nil,
		container.NewPadded(logBox),
	)
}

func buildCompletePage(state *wizardState, w fyne.Window) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(
		"Setup Complete",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	modeInfo := ""
	if state.DeploymentMode == "saas" {
		modeInfo = "\n\nSaaS/Managed Mode selected - connect to your service provider's " +
			"management console for additional configuration."
	}

	nextSteps := ""
	if state.P2PEnabled {
		nextSteps += fmt.Sprintf("\n\nP2P networking is enabled on port %s - your node will "+
			"automatically discover peers via mDNS.", state.P2PPort)
	}
	if state.MetricsEnabled {
		nextSteps += fmt.Sprintf("\n\nPrometheus metrics are available at http://localhost:%s/metrics",
			state.MetricsPort)
	}
	if state.UpdatesEnabled {
		nextSteps += "\n\nAuto-updates are enabled - your node will check for updates daily."
	}

	body := widget.NewLabel(fmt.Sprintf(
		"Your SoHoLINK node \"%s\" has been configured and initialised.%s\n\n"+
			"Data directory: %s%s\n\n"+
			"You can now close this wizard and start the node with:\n"+
			"  fedaaa server\n\n"+
			"Or launch the dashboard with:\n"+
			"  fedaaa dashboard",
		state.NodeName, modeInfo, state.DataDir, nextSteps,
	))
	body.Wrapping = fyne.TextWrapWord

	close := widget.NewButton("Finish", func() { w.Close() })
	close.Importance = widget.HighImportance

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator()),
		container.NewHBox(layout.NewSpacer(), close),
		nil, nil,
		container.NewPadded(body),
	)
}

// ---------------------------------------------------------------------------
// Main dashboard
// ---------------------------------------------------------------------------

// RunDashboard creates and displays the operator dashboard.
// The dashboard presents five tabs: Status, Users, Logs, LBTAS, and Services.
// It reads live data from the store embedded in the App instance.
func RunDashboard(application *app.App) {
	a := fyneApp.New()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("SoHoLINK Dashboard")
	w.Resize(fyne.NewSize(960, 640))
	w.CenterOnScreen()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Status", theme.HomeIcon(), buildStatusTab(application, w)),
		container.NewTabItemWithIcon("Users", theme.AccountIcon(), buildUsersTab(application, w)),
		container.NewTabItemWithIcon("Logs", theme.DocumentIcon(), buildLogsTab(application, w)),
		container.NewTabItemWithIcon("LBTAS", theme.InfoIcon(), buildLBTASTab(application, w)),
		container.NewTabItemWithIcon("Services", theme.ComputerIcon(), buildServicesTab(application, w)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.ShowAndRun()
}

// ---------------------------------------------------------------------------
// Status tab
// ---------------------------------------------------------------------------

func buildStatusTab(application *app.App, w fyne.Window) fyne.CanvasObject {
	ctx := context.Background()

	// -- Node information ---------------------------------------------------
	nodeName := safeNodeInfo(application, ctx, "node_name", application.Config.Node.Name)
	nodeDID := truncateDID(application.Config.Node.DID)
	installedAt := safeNodeInfo(application, ctx, "installed_at", "unknown")
	platform := safeNodeInfo(application, ctx, "platform", runtime.GOOS+"/"+runtime.GOARCH)

	// Uptime is approximated from installed_at when the server boot time is
	// not available.
	uptimeStr := "N/A"
	if t, err := time.Parse(time.RFC3339, installedAt); err == nil {
		uptimeStr = formatDuration(time.Since(t))
	}

	infoCard := widget.NewCard("Node Information", "", container.NewGridWithColumns(2,
		widget.NewLabel("Node Name:"), widget.NewLabel(nodeName),
		widget.NewLabel("DID:"), widget.NewLabel(nodeDID),
		widget.NewLabel("Platform:"), widget.NewLabel(platform),
		widget.NewLabel("Installed:"), widget.NewLabel(installedAt),
		widget.NewLabel("Uptime:"), widget.NewLabel(uptimeStr),
		widget.NewLabel("Data Dir:"), widget.NewLabel(application.Config.Storage.BasePath),
	))

	// -- Peer summary -------------------------------------------------------
	peerCount := 0
	if peers, err := application.Store.GetP2PPeers(ctx); err == nil {
		peerCount = len(peers)
	}
	userCount := 0
	if n, err := application.Store.ActiveUserCount(ctx); err == nil {
		userCount = n
	}

	statsCard := widget.NewCard("Quick Stats", "", container.NewGridWithColumns(2,
		widget.NewLabel("Active Users:"), widget.NewLabel(fmt.Sprintf("%d", userCount)),
		widget.NewLabel("Known Peers:"), widget.NewLabel(fmt.Sprintf("%d", peerCount)),
		widget.NewLabel("Auth Address:"), widget.NewLabel(application.Config.Radius.AuthAddress),
		widget.NewLabel("Acct Address:"), widget.NewLabel(application.Config.Radius.AcctAddress),
	))

	// -- Peer list ----------------------------------------------------------
	peerList := widget.NewList(
		func() int {
			peers, _ := application.Store.GetP2PPeers(ctx)
			return len(peers)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("DID"),
				layout.NewSpacer(),
				widget.NewLabel("Addr"),
				layout.NewSpacer(),
				widget.NewLabel("Score"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			peers, _ := application.Store.GetP2PPeers(ctx)
			if id >= len(peers) {
				return
			}
			p := peers[id]
			c := item.(*fyne.Container)
			c.Objects[0].(*widget.Label).SetText(truncateDID(p.PeerDID))
			c.Objects[2].(*widget.Label).SetText(p.Address)
			c.Objects[4].(*widget.Label).SetText(fmt.Sprintf("%d", p.Score))
		},
	)
	peerCard := widget.NewCard("Peers", "", peerList)

	// -- Refresh button -----------------------------------------------------
	refresh := widget.NewButton("Refresh", func() {
		w.SetContent(container.NewAppTabs(
			container.NewTabItemWithIcon("Status", theme.HomeIcon(), buildStatusTab(application, w)),
			container.NewTabItemWithIcon("Users", theme.AccountIcon(), buildUsersTab(application, w)),
			container.NewTabItemWithIcon("Logs", theme.DocumentIcon(), buildLogsTab(application, w)),
			container.NewTabItemWithIcon("LBTAS", theme.InfoIcon(), buildLBTASTab(application, w)),
			container.NewTabItemWithIcon("Services", theme.ComputerIcon(), buildServicesTab(application, w)),
		))
	})

	return container.NewVScroll(container.NewVBox(
		infoCard,
		statsCard,
		peerCard,
		container.NewHBox(layout.NewSpacer(), refresh),
	))
}

// ---------------------------------------------------------------------------
// Users tab
// ---------------------------------------------------------------------------

func buildUsersTab(application *app.App, w fyne.Window) fyne.CanvasObject {
	ctx := context.Background()
	s := application.Store

	// Table header
	header := container.NewGridWithColumns(5,
		widget.NewLabelWithStyle("Username", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("DID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Role", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Created", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	// Build rows
	usersBox := container.NewVBox()
	rebuildUsers := func() {
		usersBox.RemoveAll()
		users, err := s.ListUsers(ctx)
		if err != nil {
			usersBox.Add(widget.NewLabel("Error loading users: " + err.Error()))
			return
		}
		if len(users) == 0 {
			usersBox.Add(widget.NewLabel("No users registered."))
			return
		}
		for _, u := range users {
			status := "active"
			if u.RevokedAt.Valid {
				status = "revoked"
			}
			row := container.NewGridWithColumns(5,
				widget.NewLabel(u.Username),
				widget.NewLabel(truncateDID(u.DID)),
				widget.NewLabel(u.Role),
				widget.NewLabel(u.CreatedAt),
				widget.NewLabel(status),
			)
			usersBox.Add(row)
		}
	}
	rebuildUsers()

	// Add user form
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("username")
	didEntry := widget.NewEntry()
	didEntry.SetPlaceHolder("did:soho:...")
	roleSelect := widget.NewSelect([]string{"basic", "admin", "operator"}, nil)
	roleSelect.SetSelected("basic")

	addBtn := widget.NewButton("Add User", func() {
		uname := strings.TrimSpace(usernameEntry.Text)
		did := strings.TrimSpace(didEntry.Text)
		if uname == "" || did == "" {
			dialog.ShowError(fmt.Errorf("username and DID are required"), w)
			return
		}
		role := roleSelect.Selected
		if role == "" {
			role = "basic"
		}
		err := s.AddUser(ctx, uname, did, []byte{}, role)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Success", fmt.Sprintf("User %q added.", uname), w)
		usernameEntry.SetText("")
		didEntry.SetText("")
		rebuildUsers()
	})
	addBtn.Importance = widget.HighImportance

	addForm := widget.NewCard("Add User", "", container.NewVBox(
		container.NewGridWithColumns(3, usernameEntry, didEntry, roleSelect),
		addBtn,
	))

	// Revoke user form
	revokeEntry := widget.NewEntry()
	revokeEntry.SetPlaceHolder("username to revoke")
	reasonEntry := widget.NewEntry()
	reasonEntry.SetPlaceHolder("reason (optional)")

	revokeBtn := widget.NewButton("Revoke User", func() {
		uname := strings.TrimSpace(revokeEntry.Text)
		if uname == "" {
			dialog.ShowError(fmt.Errorf("username is required"), w)
			return
		}
		reason := strings.TrimSpace(reasonEntry.Text)
		if reason == "" {
			reason = "revoked via dashboard"
		}
		dialog.ShowConfirm("Confirm Revocation",
			fmt.Sprintf("Revoke user %q?\nReason: %s", uname, reason),
			func(ok bool) {
				if !ok {
					return
				}
				if err := s.RevokeUser(ctx, uname, reason); err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Revoked", fmt.Sprintf("User %q has been revoked.", uname), w)
				revokeEntry.SetText("")
				reasonEntry.SetText("")
				rebuildUsers()
			}, w)
	})
	revokeBtn.Importance = widget.DangerImportance

	revokeForm := widget.NewCard("Revoke User", "", container.NewVBox(
		container.NewGridWithColumns(2, revokeEntry, reasonEntry),
		revokeBtn,
	))

	return container.NewVScroll(container.NewVBox(
		addForm,
		revokeForm,
		widget.NewSeparator(),
		widget.NewCard("Registered Users", "", container.NewVBox(header, widget.NewSeparator(), usersBox)),
	))
}

// ---------------------------------------------------------------------------
// Logs tab (accounting log viewer)
// ---------------------------------------------------------------------------

func buildLogsTab(application *app.App, w fyne.Window) fyne.CanvasObject {
	logText := widget.NewMultiLineEntry()
	logText.Wrapping = fyne.TextWrapWord
	logText.Disable()

	loadLogs := func() {
		acctDir := application.Config.AccountingDir()
		entries, err := os.ReadDir(acctDir)
		if err != nil {
			logText.SetText(fmt.Sprintf("Cannot read accounting dir %s: %v", acctDir, err))
			return
		}

		// Sort by name descending (newest first).
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Accounting logs from: %s\n", acctDir))
		sb.WriteString(fmt.Sprintf("Found %d log file(s)\n", len(entries)))
		sb.WriteString(strings.Repeat("-", 60) + "\n")

		maxFiles := 5
		if len(entries) < maxFiles {
			maxFiles = len(entries)
		}
		for _, e := range entries[:maxFiles] {
			fp := filepath.Join(acctDir, e.Name())
			sb.WriteString(fmt.Sprintf("\n=== %s ===\n", e.Name()))
			f, err := os.Open(fp)
			if err != nil {
				sb.WriteString(fmt.Sprintf("  (cannot open: %v)\n", err))
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, 8192))
			f.Close()
			if err != nil {
				sb.WriteString(fmt.Sprintf("  (read error: %v)\n", err))
				continue
			}
			if len(data) == 0 {
				sb.WriteString("  (empty)\n")
			} else {
				sb.Write(data)
				if len(data) == 8192 {
					sb.WriteString("\n  ... (truncated) ...\n")
				}
			}
		}
		logText.SetText(sb.String())
	}

	loadLogs()

	refreshBtn := widget.NewButton("Refresh Logs", func() { loadLogs() })
	refreshBtn.Importance = widget.HighImportance

	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("Accounting Log Viewer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			refreshBtn,
		),
		nil, nil, nil,
		logText,
	)
}

// ---------------------------------------------------------------------------
// LBTAS tab (reputation scores)
// ---------------------------------------------------------------------------

func buildLBTASTab(application *app.App, w fyne.Window) fyne.CanvasObject {
	ctx := context.Background()
	s := application.Store

	// Look up an LBTAS score by DID.
	didEntry := widget.NewEntry()
	didEntry.SetPlaceHolder("Enter DID to look up...")

	resultBox := container.NewVBox()

	lookupBtn := widget.NewButton("Look Up Score", func() {
		did := strings.TrimSpace(didEntry.Text)
		if did == "" {
			dialog.ShowError(fmt.Errorf("please enter a DID"), w)
			return
		}
		score, err := s.GetLBTASScore(ctx, did)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		resultBox.RemoveAll()
		if score == nil {
			resultBox.Add(widget.NewLabel("No LBTAS score found for this DID."))
			return
		}
		resultBox.Add(container.NewGridWithColumns(2,
			widget.NewLabel("DID:"), widget.NewLabel(truncateDID(score.DID)),
			widget.NewLabel("Overall Score:"), widget.NewLabel(fmt.Sprintf("%d / 100", score.OverallScore)),
			widget.NewLabel("Payment Reliability:"), widget.NewLabel(fmt.Sprintf("%.2f", score.PaymentReliability)),
			widget.NewLabel("Execution Quality:"), widget.NewLabel(fmt.Sprintf("%.2f", score.ExecutionQuality)),
			widget.NewLabel("Communication:"), widget.NewLabel(fmt.Sprintf("%.2f", score.Communication)),
			widget.NewLabel("Resource Usage:"), widget.NewLabel(fmt.Sprintf("%.2f", score.ResourceUsage)),
			widget.NewLabel("Total Transactions:"), widget.NewLabel(fmt.Sprintf("%d", score.TotalTransactions)),
			widget.NewLabel("Completed:"), widget.NewLabel(fmt.Sprintf("%d", score.CompletedTransactions)),
			widget.NewLabel("Disputed:"), widget.NewLabel(fmt.Sprintf("%d", score.DisputedTransactions)),
			widget.NewLabel("Last Updated:"), widget.NewLabel(score.UpdatedAt.Format(time.RFC3339)),
		))
	})
	lookupBtn.Importance = widget.HighImportance

	lookupCard := widget.NewCard("LBTAS Score Lookup", "", container.NewVBox(
		container.NewBorder(nil, nil, nil, lookupBtn, didEntry),
		widget.NewSeparator(),
		resultBox,
	))

	// Show all known LBTAS scores from the database.
	allScoresBox := container.NewVBox()
	loadAllScores := func() {
		allScoresBox.RemoveAll()
		rows, err := s.DB().QueryContext(ctx,
			`SELECT did, overall_score, total_transactions, completed_transactions, disputed_transactions
			 FROM lbtas_scores ORDER BY overall_score DESC LIMIT 50`)
		if err != nil {
			allScoresBox.Add(widget.NewLabel("Error: " + err.Error()))
			return
		}
		defer rows.Close()

		header := container.NewGridWithColumns(5,
			widget.NewLabelWithStyle("DID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Score", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Total", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Completed", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Disputed", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
		allScoresBox.Add(header)
		allScoresBox.Add(widget.NewSeparator())

		count := 0
		for rows.Next() {
			var did string
			var overall, total, completed, disputed int
			if err := rows.Scan(&did, &overall, &total, &completed, &disputed); err != nil {
				continue
			}
			allScoresBox.Add(container.NewGridWithColumns(5,
				widget.NewLabel(truncateDID(did)),
				widget.NewLabel(fmt.Sprintf("%d", overall)),
				widget.NewLabel(fmt.Sprintf("%d", total)),
				widget.NewLabel(fmt.Sprintf("%d", completed)),
				widget.NewLabel(fmt.Sprintf("%d", disputed)),
			))
			count++
		}
		if count == 0 {
			allScoresBox.Add(widget.NewLabel("No LBTAS scores recorded yet."))
		}
	}
	loadAllScores()

	refreshBtn := widget.NewButton("Refresh", func() { loadAllScores() })
	allCard := widget.NewCard("All Reputation Scores", "", container.NewVBox(
		container.NewHBox(layout.NewSpacer(), refreshBtn),
		allScoresBox,
	))

	return container.NewVScroll(container.NewVBox(lookupCard, allCard))
}

// ---------------------------------------------------------------------------
// Services tab (managed services status)
// ---------------------------------------------------------------------------

func buildServicesTab(application *app.App, w fyne.Window) fyne.CanvasObject {
	ctx := context.Background()
	cfg := application.Config

	// Configuration status for managed subsystems.
	boolLabel := func(b bool) string {
		if b {
			return "Enabled"
		}
		return "Disabled"
	}

	configCard := widget.NewCard("Service Configuration", "", container.NewGridWithColumns(2,
		widget.NewLabel("Managed Services:"), widget.NewLabel(boolLabel(cfg.Services.Enabled)),
		widget.NewLabel("  PostgreSQL:"), widget.NewLabel(boolLabel(cfg.Services.Postgres)),
		widget.NewLabel("  Object Store:"), widget.NewLabel(boolLabel(cfg.Services.ObjectStore)),
		widget.NewLabel("  Message Queue:"), widget.NewLabel(boolLabel(cfg.Services.MessageQueue)),
		widget.NewSeparator(), widget.NewSeparator(),
		widget.NewLabel("CDN:"), widget.NewLabel(boolLabel(cfg.CDN.Enabled)),
		widget.NewLabel("SLA:"), widget.NewLabel(boolLabel(cfg.SLA.Enabled)),
		widget.NewLabel("Orchestration:"), widget.NewLabel(boolLabel(cfg.Orchestration.Enabled)),
		widget.NewLabel("Hypervisor:"), widget.NewLabel(boolLabel(cfg.Hypervisor.Enabled)),
		widget.NewLabel("Blockchain:"), widget.NewLabel(boolLabel(cfg.Blockchain.Enabled)),
		widget.NewLabel("P2P Mesh:"), widget.NewLabel(boolLabel(cfg.P2P.Enabled)),
		widget.NewLabel("LBTAS:"), widget.NewLabel(boolLabel(cfg.LBTAS.Enabled)),
		widget.NewLabel("Compute:"), widget.NewLabel(boolLabel(cfg.ResourceSharing.Compute.Enabled)),
		widget.NewLabel("Storage Pool:"), widget.NewLabel(boolLabel(cfg.ResourceSharing.StoragePool.Enabled)),
		widget.NewLabel("Printer Spool:"), widget.NewLabel(boolLabel(cfg.ResourceSharing.Printer.Enabled)),
		widget.NewLabel("Portal:"), widget.NewLabel(boolLabel(cfg.ResourceSharing.Portal.Enabled)),
	))

	// Service instances from the database.
	instancesBox := container.NewVBox()
	loadInstances := func() {
		instancesBox.RemoveAll()
		// We need the owner DID; use node DID as a proxy.
		ownerDID := cfg.Node.DID
		instances, err := application.Store.GetServiceInstances(ctx, ownerDID)
		if err != nil {
			instancesBox.Add(widget.NewLabel("Error loading service instances: " + err.Error()))
			return
		}
		if len(instances) == 0 {
			instancesBox.Add(widget.NewLabel("No managed service instances provisioned."))
			return
		}
		header := container.NewGridWithColumns(5,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Type", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Plan", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Endpoint", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
		instancesBox.Add(header)
		instancesBox.Add(widget.NewSeparator())
		for _, inst := range instances {
			endpoint := inst.Endpoint
			if inst.Port > 0 {
				endpoint = fmt.Sprintf("%s:%d", inst.Endpoint, inst.Port)
			}
			instancesBox.Add(container.NewGridWithColumns(5,
				widget.NewLabel(inst.Name),
				widget.NewLabel(inst.ServiceType),
				widget.NewLabel(inst.Plan),
				widget.NewLabel(inst.Status),
				widget.NewLabel(endpoint),
			))
		}
	}
	loadInstances()

	refreshBtn := widget.NewButton("Refresh", func() { loadInstances() })
	instancesCard := widget.NewCard("Service Instances", "", container.NewVBox(
		container.NewHBox(layout.NewSpacer(), refreshBtn),
		instancesBox,
	))

	// Federation nodes.
	nodesBox := container.NewVBox()
	loadNodes := func() {
		nodesBox.RemoveAll()
		nodes, err := application.Store.GetOnlineNodes(ctx)
		if err != nil {
			nodesBox.Add(widget.NewLabel("Error: " + err.Error()))
			return
		}
		if len(nodes) == 0 {
			nodesBox.Add(widget.NewLabel("No online federation nodes."))
			return
		}
		header := container.NewGridWithColumns(5,
			widget.NewLabelWithStyle("Node DID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Region", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("CPU (avail)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Mem MB (avail)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Uptime %", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)
		nodesBox.Add(header)
		nodesBox.Add(widget.NewSeparator())
		for _, n := range nodes {
			nodesBox.Add(container.NewGridWithColumns(5,
				widget.NewLabel(truncateDID(n.NodeDID)),
				widget.NewLabel(n.Region),
				widget.NewLabel(fmt.Sprintf("%.1f / %.1f", n.AvailableCPU, n.TotalCPU)),
				widget.NewLabel(fmt.Sprintf("%d / %d", n.AvailableMemoryMB, n.TotalMemoryMB)),
				widget.NewLabel(fmt.Sprintf("%.1f%%", n.UptimePercent)),
			))
		}
	}
	loadNodes()

	nodesRefresh := widget.NewButton("Refresh", func() { loadNodes() })
	nodesCard := widget.NewCard("Federation Nodes (Online)", "", container.NewVBox(
		container.NewHBox(layout.NewSpacer(), nodesRefresh),
		nodesBox,
	))

	return container.NewVScroll(container.NewVBox(configCard, instancesCard, nodesCard))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// portFromAddr extracts the port from an address string like "0.0.0.0:1812".
func portFromAddr(addr, fallback string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 && idx < len(addr)-1 {
		return addr[idx+1:]
	}
	return fallback
}

// maskSecret returns a partially-masked version of a secret string.
func maskSecret(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// truncateDID shortens a DID for display. e.g. "did:soho:abc123...xyz789".
func truncateDID(did string) string {
	if len(did) <= 24 {
		return did
	}
	return did[:16] + "..." + did[len(did)-6:]
}

// formatDuration produces a human-friendly duration string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// safeNodeInfo reads a value from node_info, returning a fallback on any error.
func safeNodeInfo(application *app.App, ctx context.Context, key, fallback string) string {
	if application.Store == nil {
		return fallback
	}
	val, err := application.Store.GetNodeInfo(ctx, key)
	if err != nil || val == "" {
		return fallback
	}
	return val
}
