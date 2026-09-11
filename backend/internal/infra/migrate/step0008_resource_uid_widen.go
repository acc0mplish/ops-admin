package migrate

import (
	"fmt"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/model"
)

// step0008ResourceUIDWiden — version 8 (I10 J1c, plan §3.3 r3): widen
// provider_task.resource_uid from varchar(64) to varchar(255).
//
// Why: the §3.3 synthetic resource_uid
// conn:<connectionUID>:<singular>:<target> reaches 181+ chars at the
// DNS-1123 maximums (32-hex connection segment + 15-char singular + 63-char
// namespace + "/" + 63-char name) — a varchar(64) column physically cannot
// hold it. Strict-mode inserts would error; non-strict silently truncates,
// and two distinct targets colliding into one truncated uid would poison the
// (resource_uid, active_flag) active-lock uniqueness.
//
// step0003 pattern (W-4/W-5 contract):
//   - AutoMigrate brings brand-new databases straight to 255 (the model tag
//     is size:255 now) — the conditional DDL below is a no-op there.
//   - existing MySQL databases get the existence-checked MODIFY COLUMN;
//     sqlite (the test runner) does not enforce varchar widths — no-op.
//     The length contract on sqlite is owned by the Go builder guard
//     (api/v2 connectionResourceUID, budget 255 — CI17).
//   - this step touches provider_task, so the EnsureProviderTaskGuards
//     postlude RE-RUNS after its own DDL (W-5: the (resource_uid,
//     active_flag) unique must be re-confirmed on top of the widened
//     column; every statement is existence-checked, so re-application is a
//     no-op when the guards survived).
var step0008ResourceUIDWiden = Step{
	Version: 8,
	Name:    "resource_uid_widen",
	Run: func(db *gorm.DB) error {
		if err := db.AutoMigrate(&model.ProviderTask{}); err != nil {
			return fmt.Errorf("migrate: step 0008 AutoMigrate provider_task: %w", err)
		}
		if db.Dialector.Name() == dialectMySQL {
			// 존재 확인 조건부 DDL(step0003 패턴): MySQL에서만 칼럼 폭을
			// 조회해 255 미만일 때만 widening한다 — 신규 DB(step0002가 갱신된
			// tag로 직접 255 생성)는 DDL 0회의 진짜 no-op이다. 실측(라이브
			// dev DB): 기존 resource_uid는 varchar(64)·nullable·no default다.
			// MODIFY는 칼럼 정의 전체를 교체하므로 nullable을 그대로 유지하는
			// 이 정의이며, 해당 칼럼의 인덱스(단일·uq_provider_task_resource_active
			// 복합)를 보존한다.
			var width int64
			if err := db.Raw(
				"SELECT COALESCE(MAX(character_maximum_length), 0) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
				"provider_task", "resource_uid",
			).Scan(&width).Error; err != nil {
				return fmt.Errorf("migrate: step 0008 resource_uid width probe: %w", err)
			}
			if width != 0 && width < 255 {
				if res := db.Exec("ALTER TABLE provider_task MODIFY COLUMN resource_uid VARCHAR(255)"); res.Error != nil {
					return fmt.Errorf("migrate: step 0008 widen provider_task.resource_uid to varchar(255): %w", res.Error)
				}
			}
		}
		if err := EnsureProviderTaskGuards(db); err != nil {
			return fmt.Errorf("migrate: step 0008 guard postlude: %w", err)
		}
		return nil
	},
}
