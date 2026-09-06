package migrate

import (
	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// step0002Tasks creates the §13.1 task-engine tables (plan §2 PR 17, file 2):
// provider_task (base shape — approval/idempotency/uniqueness columns arrive
// with the PR 18 Expand step 0003), task_attempt, task_event. Frozen at PR 17
// merge (W-4); later schema changes land as step 0004+.
var step0002Tasks = Step{
	Version: 2,
	Name:    "tasks",
	Run: func(db *gorm.DB) error {
		return db.AutoMigrate(
			&model.ProviderTask{},
			&model.TaskAttempt{},
			&model.TaskEvent{},
		)
	},
}
