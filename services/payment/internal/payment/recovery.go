package payment

import (
	"context"
	"log"
	"time"
)

const (
	// stuckThreshold is how long a payment can sit in PENDING or PROCESSING
	// before the recovery job considers it stuck and retries it.
	stuckThreshold = 5 * time.Minute

	// recoveryInterval is how often the recovery job scans for stuck payments.
	recoveryInterval = 2 * time.Minute

	// maxRetries is the number of recovery attempts before a stuck payment is
	// abandoned and marked FAILED. Set to 1 because a stuck PROCESSING record
	// indicates a system crash (not a transient error) — one retry is enough to
	// complete interrupted work. If it crashes again, something is systematically
	// wrong and the user should be notified rather than retried indefinitely.
	maxRetries = 1
)

// RecoveryJob periodically scans for payments stuck in PENDING or PROCESSING
// and retries them. This handles crashes that occur between status transitions.
//
// It is safe to run alongside normal payment processing because:
//   - ProcessPayment is idempotent for PROCESSING records (gateway_decision
//     ensures the same outcome is used on retry).
//   - The stuckThreshold gives in-flight payments time to complete normally
//     before the recovery job touches them.
type RecoveryJob struct {
	repo *Repository
	svc  *Service
}

// NewRecoveryJob creates a new RecoveryJob.
func NewRecoveryJob(repo *Repository, svc *Service) *RecoveryJob {
	return &RecoveryJob{repo: repo, svc: svc}
}

// Start runs the recovery loop in a background goroutine.
// Stops when ctx is cancelled.
func (j *RecoveryJob) Start(ctx context.Context) {
	go j.run(ctx)
}

func (j *RecoveryJob) run(ctx context.Context) {
	ticker := time.NewTicker(recoveryInterval)
	defer ticker.Stop()

	log.Printf("payment recovery: started (scan every %v, stuck threshold %v)", recoveryInterval, stuckThreshold)

	for {
		select {
		case <-ctx.Done():
			log.Println("payment recovery: stopped")
			return
		case <-ticker.C:
			j.recover(ctx)
		}
	}
}

func (j *RecoveryJob) recover(ctx context.Context) {
	cutoff := time.Now().Add(-stuckThreshold)
	stuck, err := j.repo.ScanStuck(ctx, cutoff)
	if err != nil {
		log.Printf("payment recovery: scan error: %v", err)
		return
	}
	if len(stuck) == 0 {
		return
	}

	log.Printf("payment recovery: found %d stuck payment(s)", len(stuck))
	for _, p := range stuck {
		if p.RetryCount >= maxRetries {
			// Retry budget exhausted: abandon the payment and notify the user via
			// the payment_failed event rather than retrying indefinitely.
			log.Printf("payment recovery: abandoning payment %s after %d attempt(s)", p.PaymentID, p.RetryCount)
			if err := j.svc.AbandonPayment(ctx, p.PaymentID, "payment could not be completed after retries"); err != nil {
				log.Printf("payment recovery: abandon failed for payment %s: %v", p.PaymentID, err)
			}
			continue
		}

		// Increment retry_count before attempting so that if this process crashes
		// mid-retry, the next scan sees the updated count and enforces the cap.
		if err := j.repo.IncrementRetryCount(ctx, p.PaymentID); err != nil {
			log.Printf("payment recovery: could not increment retry count for %s: %v", p.PaymentID, err)
			continue
		}
		log.Printf("payment recovery: retrying payment %s (attempt %d/%d, status=%s)",
			p.PaymentID, p.RetryCount+1, maxRetries, p.Status)
		if err := j.svc.ProcessPayment(ctx, p.PaymentID); err != nil {
			log.Printf("payment recovery: retry failed for payment %s: %v", p.PaymentID, err)
		}
	}
}
