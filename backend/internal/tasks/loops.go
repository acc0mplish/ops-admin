package tasks

import (
	"context"
	"time"
)

// Start launches the poller and reaper goroutines (§13.1 "workers are
// in-process goroutines claiming via a poller"). The derived context cancels
// with the caller's; Stop is the symmetric stop. Phase 1 starts the engine
// from tests only — main.go wiring is Phase 3's (A6, claim 16/17).
//
// Loop errors stay in the loops: task-level failures are recorded on the
// task rows; a database-level error has no caller to return to, and killing
// the loops would silently stop recovery — the loops stay up and retry next
// tick.
func (e *Engine) Start(ctx context.Context) {
	e.loopMu.Lock()
	defer e.loopMu.Unlock()
	if e.loopRun {
		return // already running — Start is idempotent
	}
	loopCtx, cancel := context.WithCancel(ctx)
	e.loopCtx = loopCtx
	e.loopStop = cancel
	e.loopRun = true

	interval := e.cfg.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second // §13.1 "default interval 2s, configurable"
	}

	e.loopWG.Add(2)
	go e.pollLoop(loopCtx, interval)
	go e.reapLoop(loopCtx, interval)
}

// Stop is the symmetric stop (r2 F-N): it signals both loops and waits for
// the in-flight RunOnce/ReapOnce cycle to complete before returning — no
// goroutine outlives the call (T39's leak assertion, PR 19). Idempotent.
func (e *Engine) Stop() {
	e.loopMu.Lock()
	cancel := e.loopStop
	e.loopMu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.loopWG.Wait()

	e.loopMu.Lock()
	e.loopRun = false
	e.loopStop = nil
	e.loopMu.Unlock()
}

func (e *Engine) pollLoop(ctx context.Context, interval time.Duration) {
	defer e.loopWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = e.RunOnce(ctx)
		}
	}
}

func (e *Engine) reapLoop(ctx context.Context, interval time.Duration) {
	defer e.loopWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = e.ReapOnce(ctx)
		}
	}
}

// RunOnce is one deterministic poller cycle (§3.6): async in-flight attempts
// are polled to completion first (no new attempts — §14.3 UPID dual mode),
// then at most one queued task is claimed and executed. Returns processed =
// true when a queued task was claimed and run through its executor.
func (e *Engine) RunOnce(ctx context.Context) (bool, error) {
	if err := e.pollAsyncAttempts(ctx); err != nil {
		return false, err
	}
	claim, ok, err := e.ClaimNext(ctx)
	if err != nil || !ok {
		return false, err
	}
	return true, e.executeClaimed(ctx, claim)
}
