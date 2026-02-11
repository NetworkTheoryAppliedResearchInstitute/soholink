package printer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// PrintJob describes a job to be sent to a printer.
type PrintJob struct {
	JobID         string
	TransactionID string
	UserDID       string
	ProviderDID   string
	PrinterType   string // "2d" or "3d"
	GCodePath     string // For 3D printers
	DocumentPath  string // For 2D printers
	Copies        int
	EstimatedGrams float64 // Filament estimate for 3D
	EstimatedPages int     // Page count for 2D
}

// PrintResult holds the outcome of a completed print job.
type PrintResult struct {
	JobID          string
	Status         string // "completed", "failed", "cancelled"
	ActualGrams    float64
	ActualPages    int
	PrintDuration  time.Duration
}

// Spooler manages the print queue and dispatches jobs to printers.
type Spooler struct {
	store      *store.Store
	accounting *accounting.Collector
	validator  *GCodeValidator
	queue      chan PrintJob
	mu         sync.Mutex
	active     map[string]*PrintJob // Active jobs by ID
}

// NewSpooler creates a new print spooler.
func NewSpooler(s *store.Store, ac *accounting.Collector, validator *GCodeValidator) *Spooler {
	return &Spooler{
		store:      s,
		accounting: ac,
		validator:  validator,
		queue:      make(chan PrintJob, 50),
		active:     make(map[string]*PrintJob),
	}
}

// SubmitJob validates and enqueues a print job.
func (sp *Spooler) SubmitJob(ctx context.Context, job PrintJob) error {
	// Validate G-code for 3D print jobs
	if job.PrinterType == "3d" && job.GCodePath != "" {
		if err := sp.validator.Validate(job.GCodePath); err != nil {
			sp.accounting.Record(&accounting.AccountingEvent{
				Timestamp: time.Now(),
				EventType: "print_job_rejected",
				UserDID:   job.UserDID,
				SessionID: job.TransactionID,
				Reason:    err.Error(),
			})
			return fmt.Errorf("G-code validation failed: %w", err)
		}
	}

	sp.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "print_job_submitted",
		UserDID:   job.UserDID,
		SessionID: job.TransactionID,
		Resource:  job.PrinterType,
	})

	select {
	case sp.queue <- job:
		return nil
	default:
		return fmt.Errorf("print queue full")
	}
}

// Run processes the print queue. Blocks until ctx is cancelled.
func (sp *Spooler) Run(ctx context.Context) {
	log.Printf("[printer] spooler started")
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-sp.queue:
			sp.processJob(ctx, job)
		}
	}
}

func (sp *Spooler) processJob(ctx context.Context, job PrintJob) {
	sp.mu.Lock()
	sp.active[job.JobID] = &job
	sp.mu.Unlock()

	defer func() {
		sp.mu.Lock()
		delete(sp.active, job.JobID)
		sp.mu.Unlock()
	}()

	log.Printf("[printer] processing job %s (type=%s)", job.JobID, job.PrinterType)

	// In production, this would communicate with the actual printer hardware.
	// For now, just log the event.

	sp.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "print_job_completed",
		UserDID:   job.UserDID,
		SessionID: job.TransactionID,
		Resource:  job.PrinterType,
	})

	log.Printf("[printer] job %s completed", job.JobID)
}

// QueueSize returns the current number of jobs waiting in the queue.
func (sp *Spooler) QueueSize() int {
	return len(sp.queue)
}

// ActiveJobs returns the number of currently processing jobs.
func (sp *Spooler) ActiveJobs() int {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return len(sp.active)
}
