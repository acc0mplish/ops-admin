package tasks

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// Idempotent submit recovery (§13.4 / N6): a submit carrying an
// IdempotencyKey that already exists on a provider_task row is a REPLAY, not
// a new execution — the engine returns the existing task with replayed=true
// and writes nothing.
//
// Resolution order (Submit, R4 정합 패턴 — insert-first, recover-after):
// when the insert transaction fails for ANY reason and the submit carried a
// key, the engine first looks the key up; a hit means the insert was a
// genuine key duplicate (the unique index fired — possibly alongside the
// resource-active unique when the replay targets the same resource, in which
// case replay wins: N6 answers "has this already run?", not "is the resource
// free?"). A miss falls through to the ordinary violation classification
// (resource_busy et al.). The lookup runs AFTER the transaction rolled back —
// recovering inside it would read the conflicting row through a transaction
// that just failed.

// replayByIdempotencyKey recovers the task holding key. found=false with a
// nil error is impossible by construction; a lookup miss surfaces as an error
// (the conflicting insert's transaction is still in flight concurrently — the
// caller retries, R4).
func (e *Engine) replayByIdempotencyKey(ctx context.Context, key string) (existing model.ProviderTask, found bool, err error) {
	var task model.ProviderTask
	err = e.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&task).Error
	if err == nil {
		return task, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ProviderTask{}, false, fmt.Errorf(
			"tasks: idempotency key %q collided but no committed task holds it (concurrent submit in flight — retry, R4)", key)
	}
	return model.ProviderTask{}, false, fmt.Errorf("tasks: recover idempotent task by key %q: %w", key, err)
}
