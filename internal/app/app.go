package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crypto/ed25519"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/blockchain"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/cdn"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/central"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/compute"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/config"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/httpapi"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/lbtas"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/merkle"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/orchestration"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/payment"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/policy"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/portal"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/printer"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/radius"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/rental"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/services"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/sla"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/storage"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/thinclient"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/verifier"
)

// App is the main application lifecycle manager.
// It wires together all subsystems and manages startup/shutdown.
type App struct {
	Config     *config.Config
	Store      *store.Store
	Verifier   *verifier.Verifier
	PolicyEng  *policy.Engine
	Accounting *accounting.Collector
	Batcher    *merkle.Batcher
	Radius     *radius.Server

	// Resource sharing subsystems (nil when disabled)
	LBTASManager    *lbtas.Manager
	PaymentLedger   *payment.Ledger
	ComputeSched    *compute.Scheduler
	StoragePool     *storage.Pool
	PrintSpooler    *printer.Spooler
	PortalServer    *portal.Server
	HTTPAPIServer   *httpapi.Server
	TimeoutResolver *lbtas.TimeoutResolver

	// Central SOHO subsystems (nil when disabled)
	Notifier           *central.Notifier
	CapacityMonitor    *central.CapacityMonitor
	TenantManager      *central.TenantManager
	RatingMonitor      *central.RatingMonitor
	DisputeManager     *central.DisputeManager
	CenterRatingMgr    *central.CenterRatingManager

	// P2P mesh fallback (nil when disabled)
	P2PNetwork *thinclient.P2PNetwork

	// Rental management (nil when disabled)
	RentalEngine *rental.AutoAcceptEngine

	// Blockchain anchoring (nil when disabled)
	LocalChain *blockchain.LocalChain

	// Enterprise architecture subsystems (nil when disabled)
	FedScheduler    *orchestration.FedScheduler
	ServiceCatalog  *services.Catalog
	CDNRouter       *cdn.Router
	SLAMonitor      *sla.Monitor
	SLARecommender  *sla.Recommender
	HypervisorMgr   *compute.HypervisorManager

	cancelFunc context.CancelFunc
}

// New creates a new App instance, initializing all subsystems.
func New(cfg *config.Config) (*App, error) {
	app := &App{Config: cfg}

	// Initialize store
	s, err := store.NewStore(cfg.DatabasePath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}
	app.Store = s

	// Initialize verifier
	credTTL := time.Duration(cfg.Auth.CredentialTTL) * time.Second
	maxNonceAge := time.Duration(cfg.Auth.MaxNonceAge) * time.Second
	if maxNonceAge == 0 {
		maxNonceAge = credTTL
	}
	if credTTL > 0 && maxNonceAge < credTTL {
		log.Printf("[app] auth.max_nonce_age (%s) is shorter than credential_ttl (%s); extending nonce cache duration to match TTL",
			maxNonceAge, credTTL)
		maxNonceAge = credTTL
	}
	if maxNonceAge > 0 {
		cfg.Auth.MaxNonceAge = int(maxNonceAge.Seconds())
	}
	app.Verifier = verifier.NewVerifier(s, credTTL, maxNonceAge)

	// Initialize policy engine
	pe, err := policy.NewEngine(cfg.Policy.Directory)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to initialize policy engine: %w", err)
	}
	app.PolicyEng = pe

	// Initialize accounting collector
	ac, err := accounting.NewCollector(cfg.AccountingDir())
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to initialize accounting collector: %w", err)
	}
	app.Accounting = ac

	// Initialize Merkle batcher
	batchInterval, err := time.ParseDuration(cfg.Merkle.BatchInterval)
	if err != nil {
		batchInterval = 1 * time.Hour
	}
	app.Batcher = merkle.NewBatcher(cfg.AccountingDir(), cfg.MerkleDir(), batchInterval)

	// Initialize RADIUS server
	app.Radius = radius.NewServer(
		cfg.Radius.AuthAddress,
		cfg.Radius.AcctAddress,
		cfg.Radius.SharedSecret,
		app.Verifier,
		app.PolicyEng,
		app.Accounting,
	)

	// Initialize resource sharing subsystems (if enabled)
	if cfg.ResourceSharing.Enabled {
		if err := app.initResourceSharing(cfg); err != nil {
			log.Printf("[app] resource sharing init error (non-fatal): %v", err)
		}
	}

	// Initialize central SOHO subsystems (if enabled)
	if cfg.Central.Enabled {
		app.initCentral(cfg)
	}

	// Initialize P2P mesh (if enabled)
	if cfg.P2P.Enabled {
		app.initP2P(cfg)
	}

	// Initialize rental management (if enabled)
	if cfg.Rental.Enabled {
		app.initRental()
	}

	// Initialize enterprise architecture subsystems
	if cfg.Orchestration.Enabled {
		app.initOrchestration()
	}
	if cfg.Services.Enabled {
		app.initManagedServices(cfg)
	}
	if cfg.CDN.Enabled {
		app.initCDN()
	}
	if cfg.SLA.Enabled {
		app.initSLA()
	}
	if cfg.Hypervisor.Enabled {
		app.initHypervisor()
	}
	if cfg.Blockchain.Enabled {
		app.initBlockchain(cfg)
	}

	return app, nil
}

// initBlockchain initializes the local blockchain for Merkle root anchoring.
func (a *App) initBlockchain(cfg *config.Config) {
	log.Printf("[app] initializing local blockchain")

	// Load node private key for block signing
	var privKey ed25519.PrivateKey
	keyPath := cfg.NodeKeyPath()
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == ed25519.PrivateKeySize {
		privKey = ed25519.PrivateKey(keyData)
	}

	a.LocalChain = blockchain.NewLocalChain(a.Store, cfg.Node.DID, privKey)
	log.Printf("[app] local blockchain initialized (DID=%s)", cfg.Node.DID)
}

// initResourceSharing initializes all resource sharing subsystems.
func (a *App) initResourceSharing(cfg *config.Config) error {
	log.Printf("[app] initializing resource sharing subsystems")

	// Initialize LBTAS manager
	a.LBTASManager = lbtas.NewManager(a.Store, a.Accounting)

	// Initialize payment processors
	var processors []payment.PaymentProcessor
	for _, pc := range cfg.Payment.Processors {
		switch pc.Type {
		case "barter":
			processors = append(processors, payment.NewBarterProcessor(pc.FederationOnly))
		case "stripe":
			secretKey := os.Getenv(pc.SecretKeyEnv)
			processors = append(processors, payment.NewStripeProcessor(secretKey, ""))
		case "lightning":
			processors = append(processors, payment.NewLightningProcessor(pc.LNDHost, ""))
		case "federation_token":
			processors = append(processors, payment.NewFederationTokenProcessor(pc.Contract, ""))
		}
	}
	a.PaymentLedger = payment.NewLedger(a.Store, processors)

	// Initialize timeout resolver
	checkInterval := 1 * time.Hour
	if cfg.LBTAS.TimeoutCheckInterval != "" {
		if parsed, err := time.ParseDuration(cfg.LBTAS.TimeoutCheckInterval); err == nil {
			checkInterval = parsed
		}
	}
	a.TimeoutResolver = lbtas.NewTimeoutResolver(a.Store, a.Accounting, checkInterval)

	// Initialize compute scheduler
	if cfg.ResourceSharing.Compute.Enabled {
		workers := cfg.ResourceSharing.Compute.Workers
		if workers <= 0 {
			workers = 2
		}
		sandbox := compute.NewSandbox(cfg.ComputeWorkDir())
		a.ComputeSched = compute.NewScheduler(a.Store, a.Accounting, a.LBTASManager, sandbox, workers)
	}

	// Initialize storage pool
	if cfg.ResourceSharing.StoragePool.Enabled {
		var scanner *storage.ContentScanner
		if cfg.ResourceSharing.StoragePool.ContentScanning {
			scanner = storage.NewContentScanner(cfg.ResourceSharing.StoragePool.ClamAVSocket)
		}
		maxFileSize := cfg.ResourceSharing.StoragePool.MaxFileSize
		if maxFileSize <= 0 {
			maxFileSize = 10 * 1024 * 1024 * 1024 // 10 GB default
		}
		a.StoragePool = storage.NewPool(a.Store, a.Accounting, scanner, cfg.StoragePoolDir(), maxFileSize)
	}

	// Initialize printer spooler
	if cfg.ResourceSharing.Printer.Enabled {
		validator := printer.NewGCodeValidator()
		if cfg.ResourceSharing.Printer.MaxHotendTemp > 0 {
			validator.MaxTemp = cfg.ResourceSharing.Printer.MaxHotendTemp
		}
		if cfg.ResourceSharing.Printer.MaxBedTemp > 0 {
			validator.MaxBedTemp = cfg.ResourceSharing.Printer.MaxBedTemp
		}
		if cfg.ResourceSharing.Printer.MaxFeedRate > 0 {
			validator.MaxFeedRate = cfg.ResourceSharing.Printer.MaxFeedRate
		}
		a.PrintSpooler = printer.NewSpooler(a.Store, a.Accounting, validator)
	}

	// Initialize captive portal
	if cfg.ResourceSharing.Portal.Enabled {
		portalAddr := cfg.ResourceSharing.PortalAddress
		if portalAddr == "" {
			portalAddr = "0.0.0.0:8081"
		}
		a.PortalServer = portal.NewServer(a.Store, a.Accounting, a.Verifier, portalAddr)
	}

	// Initialize HTTP API server
	apiAddr := cfg.ResourceSharing.HTTPAPIAddress
	if apiAddr == "" {
		apiAddr = "0.0.0.0:8080"
	}
	a.HTTPAPIServer = httpapi.NewServer(a.Store, a.LBTASManager, apiAddr)

	log.Printf("[app] resource sharing subsystems initialized")
	return nil
}

// initCentral initializes central SOHO operator subsystems:
// capacity monitoring, tenant management, rating alerts, disputes, center rating.
func (a *App) initCentral(cfg *config.Config) {
	log.Printf("[app] initializing central SOHO subsystems")

	a.Notifier = central.NewNotifier()
	a.TenantManager = central.NewTenantManager(a.Store)

	// Capacity monitor
	interval := 5 * time.Minute
	if cfg.Central.CapacityCheckInterval != "" {
		if parsed, err := time.ParseDuration(cfg.Central.CapacityCheckInterval); err == nil {
			interval = parsed
		}
	}
	a.CapacityMonitor = central.NewCapacityMonitor(a.Store, interval)

	// Dispute manager (needs payment ledger)
	a.DisputeManager = central.NewDisputeManager(a.Store, a.Notifier, a.PaymentLedger)

	// Rating monitor (needs dispute manager)
	a.RatingMonitor = central.NewRatingMonitor(a.Store, a.Notifier, a.DisputeManager, cfg.Central.CenterDID)

	// Center rating manager
	a.CenterRatingMgr = central.NewCenterRatingManager(a.Store, a.Notifier)

	log.Printf("[app] central SOHO subsystems initialized (center DID=%s)", cfg.Central.CenterDID)
}

// initP2P initializes the thin-client P2P mesh networking fallback.
func (a *App) initP2P(cfg *config.Config) {
	log.Printf("[app] initializing P2P mesh network")

	listenAddr := cfg.P2P.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0:9090"
	}

	a.P2PNetwork = thinclient.NewP2PNetwork(a.Store, cfg.Node.DID, listenAddr)
	log.Printf("[app] P2P mesh initialized (listen=%s)", listenAddr)
}

// initRental initializes the rental management and auto-accept engine.
func (a *App) initRental() {
	log.Printf("[app] initializing rental management")
	a.RentalEngine = rental.NewAutoAcceptEngine(a.Store)
	log.Printf("[app] rental engine initialized")
}

// initOrchestration initializes the elastic orchestration system (FedScheduler).
func (a *App) initOrchestration() {
	log.Printf("[app] initializing elastic orchestration")
	a.FedScheduler = orchestration.NewFedScheduler(a.Store)
	log.Printf("[app] FedScheduler initialized")
}

// initManagedServices initializes the managed services catalog and provisioners.
func (a *App) initManagedServices(cfg *config.Config) {
	log.Printf("[app] initializing managed services catalog")
	a.ServiceCatalog = services.NewCatalog(a.Store)

	if cfg.Services.Postgres {
		a.ServiceCatalog.RegisterProvisioner(services.ServiceTypePostgres, services.NewPostgresProvisioner())
	}
	if cfg.Services.ObjectStore {
		a.ServiceCatalog.RegisterProvisioner(services.ServiceTypeObjectStore, services.NewObjectStoreProvisioner())
	}
	if cfg.Services.MessageQueue {
		a.ServiceCatalog.RegisterProvisioner(services.ServiceTypeMessageQueue, services.NewQueueProvisioner())
	}
	log.Printf("[app] managed services catalog initialized")
}

// initCDN initializes the CDN routing and edge cache layer.
func (a *App) initCDN() {
	log.Printf("[app] initializing CDN router")
	a.CDNRouter = cdn.NewRouter()
	log.Printf("[app] CDN router initialized")
}

// initSLA initializes the SLA monitoring and recommendation engine.
func (a *App) initSLA() {
	log.Printf("[app] initializing SLA subsystem")
	a.SLAMonitor = sla.NewMonitor(a.Store)
	a.SLARecommender = sla.NewRecommender()
	log.Printf("[app] SLA monitor and recommender initialized")
}

// initHypervisor initializes the bare-metal hypervisor manager.
func (a *App) initHypervisor() {
	log.Printf("[app] initializing hypervisor manager")
	a.HypervisorMgr = compute.NewHypervisorManager()
	if a.HypervisorMgr.Available() {
		log.Printf("[app] hypervisor available: %v", a.HypervisorMgr.ListBackends())
	} else {
		log.Printf("[app] no hypervisor backends available on this platform")
	}
}

// Start begins all services and blocks until a shutdown signal is received.
func (a *App) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel

	// Start RADIUS server
	if err := a.Radius.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start RADIUS server: %w", err)
	}

	// Start Merkle batcher in background with blockchain anchoring callback
	if a.LocalChain != nil {
		a.Batcher.SetAnchorFunc(func(rootHash []byte, sourceFile string, leafCount, treeHeight int) {
			_, _, err := a.LocalChain.SubmitBatch(ctx, rootHash, blockchain.BatchMetadata{
				SourceFile: sourceFile,
				LeafCount:  leafCount,
				TreeHeight: treeHeight,
				NodeDID:    a.Config.Node.DID,
			})
			if err != nil {
				log.Printf("[app] blockchain anchor failed: %v", err)
			}
		})
	}
	go a.Batcher.Start(ctx)

	// Start nonce pruner in background
	go a.startNoncePruner(ctx)

	// Start log compressor in background
	go a.startLogCompressor(ctx)

	// Start resource sharing subsystems (if enabled)
	if a.Config.ResourceSharing.Enabled {
		a.startResourceSharing(ctx)
	}

	// Start central SOHO subsystems (if enabled)
	if a.Config.Central.Enabled {
		a.startCentral(ctx)
	}

	// Start P2P mesh (if enabled)
	if a.P2PNetwork != nil {
		a.P2PNetwork.Start(ctx)
		log.Printf("[app]   P2P mesh:   %s", a.Config.P2P.ListenAddr)
	}

	// Load rental rules (if enabled)
	if a.RentalEngine != nil {
		if err := a.RentalEngine.LoadRules(ctx); err != nil {
			log.Printf("[app] rental rules load error: %v", err)
		}
		log.Printf("[app]   Rental:     enabled")
	}

	// Start enterprise architecture subsystems
	if a.FedScheduler != nil {
		a.FedScheduler.Start(ctx)
		log.Printf("[app]   Orchestrator: FedScheduler started")
	}
	if a.ServiceCatalog != nil {
		go a.ServiceCatalog.HealthCheckLoop(ctx)
		log.Printf("[app]   Services:   catalog started")
	}
	if a.CDNRouter != nil {
		done := make(chan struct{})
		go func() {
			<-ctx.Done()
			close(done)
		}()
		go a.CDNRouter.HealthCheckLoop(done)
		log.Printf("[app]   CDN:        router started")
	}
	if a.SLAMonitor != nil {
		go a.SLAMonitor.MonitorLoop(ctx)
		log.Printf("[app]   SLA:        monitor started")
	}

	log.Printf("[app] SoHoLINK AAA node started")
	log.Printf("[app]   Auth:       %s", a.Radius.AuthAddr())
	log.Printf("[app]   Accounting: %s", a.Radius.AcctAddr())
	log.Printf("[app]   Data dir:   %s", a.Config.Storage.BasePath)
	log.Printf("[app]   Policies:   %s", a.Config.Policy.Directory)
	if a.Config.ResourceSharing.Enabled {
		log.Printf("[app]   HTTP API:   %s", a.Config.ResourceSharing.HTTPAPIAddress)
		if a.Config.ResourceSharing.Compute.Enabled {
			log.Printf("[app]   Compute:    %d workers", a.Config.ResourceSharing.Compute.Workers)
		}
		if a.Config.ResourceSharing.StoragePool.Enabled {
			log.Printf("[app]   Storage:    %s", a.Config.StoragePoolDir())
		}
		if a.Config.ResourceSharing.Printer.Enabled {
			log.Printf("[app]   Printer:    enabled")
		}
		if a.Config.ResourceSharing.Portal.Enabled {
			log.Printf("[app]   Portal:     %s", a.Config.ResourceSharing.PortalAddress)
		}
	}
	if a.Config.Central.Enabled {
		log.Printf("[app]   Central:    enabled (DID=%s)", a.Config.Central.CenterDID)
	}
	if a.Config.Orchestration.Enabled {
		log.Printf("[app]   Orchestration: enabled")
	}
	if a.Config.Services.Enabled {
		log.Printf("[app]   Services:   managed (pg=%v, s3=%v, mq=%v)",
			a.Config.Services.Postgres, a.Config.Services.ObjectStore, a.Config.Services.MessageQueue)
	}
	if a.Config.CDN.Enabled {
		log.Printf("[app]   CDN:        enabled (cache=%dMB)", a.Config.CDN.CacheCapacityMB)
	}
	if a.Config.SLA.Enabled {
		log.Printf("[app]   SLA:        enabled (tier=%s)", a.Config.SLA.DefaultTier)
	}
	if a.Config.Hypervisor.Enabled {
		log.Printf("[app]   Hypervisor: enabled (backend=%s)", a.Config.Hypervisor.PreferBackend)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[app] received signal: %v, shutting down...", sig)

	return a.Shutdown()
}

// Shutdown performs an orderly shutdown of all subsystems.
func (a *App) Shutdown() error {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Stop resource sharing subsystems
	if a.HTTPAPIServer != nil {
		if err := a.HTTPAPIServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("[app] HTTP API shutdown error: %v", err)
		}
	}
	if a.ComputeSched != nil {
		a.ComputeSched.Stop()
	}

	// 1b. Stop enterprise subsystems
	if a.FedScheduler != nil {
		a.FedScheduler.Stop()
	}

	// 2. Stop accepting new RADIUS packets
	if err := a.Radius.Shutdown(shutdownCtx); err != nil {
		log.Printf("[app] RADIUS shutdown error: %v", err)
	}

	// 3. Flush accounting logs
	if err := a.Accounting.Close(); err != nil {
		log.Printf("[app] accounting close error: %v", err)
	}

	// 4. Build final Merkle batch
	if err := a.Batcher.BuildBatch(); err != nil {
		log.Printf("[app] final Merkle batch error: %v", err)
	}

	// 5. Close database
	if err := a.Store.Close(); err != nil {
		log.Printf("[app] store close error: %v", err)
	}

	log.Printf("[app] shutdown complete")
	return nil
}

// startResourceSharing launches all enabled resource sharing background services.
func (a *App) startResourceSharing(ctx context.Context) {
	// Start LBTAS timeout resolver
	if a.TimeoutResolver != nil {
		go a.TimeoutResolver.Run(ctx)
		log.Printf("[app] LBTAS timeout resolver started")
	}

	// Start compute scheduler
	if a.ComputeSched != nil {
		a.ComputeSched.Start(ctx)
	}

	// Start printer spooler
	if a.PrintSpooler != nil {
		go a.PrintSpooler.Run(ctx)
	}

	// Start captive portal
	if a.PortalServer != nil {
		go func() {
			if err := a.PortalServer.Start(ctx); err != nil {
				log.Printf("[app] portal server error: %v", err)
			}
		}()
	}

	// Start HTTP API server
	if a.HTTPAPIServer != nil {
		go func() {
			if err := a.HTTPAPIServer.Start(ctx); err != nil {
				log.Printf("[app] HTTP API server error: %v", err)
			}
		}()
	}
}

// startCentral launches all central SOHO background services.
func (a *App) startCentral(ctx context.Context) {
	// Start capacity monitor
	if a.CapacityMonitor != nil {
		go a.CapacityMonitor.Run(ctx)
		log.Printf("[app] capacity monitor started")
	}

	// Start capacity alert consumer
	if a.CapacityMonitor != nil && a.Notifier != nil {
		go func() {
			for {
				select {
				case alert := <-a.CapacityMonitor.AlertChan():
					a.Notifier.Send(central.Notification{
						Type:     "capacity_alert",
						Severity: alert.Severity,
						Title:    alert.Resource,
						Message:  alert.Message,
					})
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// startNoncePruner runs periodically to clean up expired nonces.
func (a *App) startNoncePruner(ctx context.Context) {
	maxAge := time.Duration(a.Config.Auth.MaxNonceAge) * time.Second
	if maxAge == 0 {
		maxAge = 5 * time.Minute
	}

	ticker := time.NewTicker(maxAge)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruned, err := a.Store.PruneExpiredNonces(ctx, maxAge)
			if err != nil {
				log.Printf("[app] nonce pruner error: %v", err)
			} else if pruned > 0 {
				log.Printf("[app] pruned %d expired nonces", pruned)
			}
		}
	}
}

// startLogCompressor runs periodically to compress old accounting logs.
func (a *App) startLogCompressor(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			maxAge := time.Duration(a.Config.Accounting.CompressAfterDays) * 24 * time.Hour
			if maxAge == 0 {
				maxAge = 7 * 24 * time.Hour
			}
			compressed, err := accounting.CompressOldLogs(a.Config.AccountingDir(), maxAge)
			if err != nil {
				log.Printf("[app] log compressor error: %v", err)
			} else if compressed > 0 {
				log.Printf("[app] compressed %d old log files", compressed)
			}
		}
	}
}
