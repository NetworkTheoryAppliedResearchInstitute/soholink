package compute

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/lbtas"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Scheduler accepts compute jobs, validates them, and dispatches to workers.
type Scheduler struct {
	store      *store.Store
	accounting *accounting.Collector
	sandbox    *Sandbox
	lbtas      *lbtas.Manager

	jobQueue chan ComputeJob
	workers  int
	wg       sync.WaitGroup
}

// NewScheduler creates a new compute job scheduler.
func NewScheduler(s *store.Store, ac *accounting.Collector, lm *lbtas.Manager, sandbox *Sandbox, workers int) *Scheduler {
	return &Scheduler{
		store:      s,
		accounting: ac,
		sandbox:    sandbox,
		lbtas:      lm,
		jobQueue:   make(chan ComputeJob, 100),
		workers:    workers,
	}
}

// Start launches worker goroutines. Call Stop to shut down.
func (s *Scheduler) Start(ctx context.Context) {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}
	log.Printf("[compute] scheduler started with %d workers", s.workers)
}

// Stop waits for all workers to finish.
func (s *Scheduler) Stop() {
	close(s.jobQueue)
	s.wg.Wait()
	log.Printf("[compute] scheduler stopped")
}

// SubmitJob validates and enqueues a compute job.
func (s *Scheduler) SubmitJob(ctx context.Context, job ComputeJob) (*store.ResourceTransactionRow, error) {
	// Check provider reputation
	if s.lbtas != nil {
		score, err := s.lbtas.GetScore(ctx, job.ProviderDID)
		if err != nil {
			return nil, fmt.Errorf("failed to check provider score: %w", err)
		}
		if score.OverallScore < 40 {
			return nil, fmt.Errorf("provider score %d below minimum 40", score.OverallScore)
		}
	}

	// Create transaction record using store-layer type directly
	tx := &store.ResourceTransactionRow{
		TransactionID:  job.TransactionID,
		UserDID:        job.UserDID,
		ProviderDID:    job.ProviderDID,
		ResourceType:   "compute",
		ResourceID:     job.JobID,
		State:          string(lbtas.StateInitiated),
		RatingDeadline: time.Now().Add(48 * time.Hour),
		CreatedAt:      time.Now(),
	}

	if err := s.store.CreateTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	s.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "compute_job_submitted",
		UserDID:   job.UserDID,
		SessionID: job.TransactionID,
	})

	// Enqueue job
	select {
	case s.jobQueue <- job:
	default:
		return nil, fmt.Errorf("compute queue full")
	}

	return tx, nil
}

func (s *Scheduler) worker(ctx context.Context, id int) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.jobQueue:
			if !ok {
				return
			}
			s.processJob(ctx, job, id)
		}
	}
}

func (s *Scheduler) processJob(ctx context.Context, job ComputeJob, workerID int) {
	log.Printf("[compute] worker %d processing job %s", workerID, job.JobID)

	// Update state to executing
	s.store.UpdateTransactionState(ctx, job.TransactionID, string(lbtas.StateExecuting))

	// Execute in sandbox
	result, err := s.sandbox.Execute(ctx, job)
	if err != nil {
		log.Printf("[compute] job %s failed: %v", job.JobID, err)
		s.store.UpdateTransactionState(ctx, job.TransactionID, string(lbtas.StateDisputed))
		s.accounting.Record(&accounting.AccountingEvent{
			Timestamp: time.Now(),
			EventType: "compute_job_failed",
			UserDID:   job.UserDID,
			SessionID: job.TransactionID,
			Reason:    err.Error(),
		})
		return
	}

	// Job completed - transition to awaiting provider rating
	s.store.UpdateTransactionState(ctx, job.TransactionID, string(lbtas.StateAwaitingProviderRating))

	s.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "compute_job_completed",
		UserDID:   job.UserDID,
		SessionID: job.TransactionID,
		Decision:  fmt.Sprintf("exit_code:%d,cpu_used:%d", result.ExitCode, result.CPUUsed),
	})

	log.Printf("[compute] job %s completed (exit=%d)", job.JobID, result.ExitCode)
}
