package model

import (
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// ProviderTask is the §13 durable-task-engine unit of work, assembled from
// spec §13.1–13.5 (+§16.2/§18.2) field-by-field (plan §3.4, A3). The
// approval/idempotency/uniqueness columns arrive with the PR 18 Expand
// migration (step0003) — see the block comment below.
type ProviderTask struct { // §13.1 "the unit of work + lease column + approval columns"
	ID                 uint
	UID                string           `gorm:"size:64;not null;uniqueIndex"` // §16.2 GET /tasks/{uid}
	OperationName      string           `gorm:"size:128;not null"`            // §10.2 def 이름
	OperationVersion   string           `gorm:"size:64"`
	ResourceUID        string           `gorm:"size:64;index"` // §13.4 유일성 대상
	PayloadJSON        contract.JSONMap `gorm:"serializer:json;type:text"`
	Status             string           `gorm:"size:32;not null;index"` // §13.5 상태머신
	AttemptCount       int              `gorm:"default:0"`              // §13.2 claim 증가
	MaxAttempts        int              `gorm:"default:1"`              // §13.2 "when attempts remain" 판정 (def.RetryPolicy 스냅샷)
	NextAttemptAt      *time.Time       `gorm:"index"`                  // §13.2 claim 조건
	LeaseExpiresAt     *time.Time       // §13.2 리스
	Version            int              `gorm:"default:0"`     // §13.4 낙관적 동시성 — 모든 쓰기 WHERE version=?
	CancelRequested    bool             `gorm:"default:false"` // §13.4 "requested via status flag"
	ErrorCode          string           `gorm:"size:128"`      // §13.2 'lease_expired' / §13.4 'resource_busy'
	ErrorMessage       string           `gorm:"type:text"`
	CallTimeoutSeconds int              `gorm:"default:0"` // §13.4 provider call timeout (per attempt)
	DeadlineAt         *time.Time       // §13.4 task deadline (overall)
	// step0003 (PR 18) Expand — 이 시점엔 미포함:
	//   RequiresApproval  bool   `gorm:"default:false"`                      // §13.3 전이 판단 스냅샷
	//   ApprovalStatus    string `gorm:"size:32;default:not_required;index"` // §13.3 OpsJob 어휘 (model/ops.go:538 관례)
	//   Approver          string `gorm:"size:128"`                           // §13.3
	//   ApprovalAt        *time.Time                                             // §13.3
	//   IdempotencyKey    *string `gorm:"size:128;uniqueIndex:uq_provider_task_idempotency"` // §13.4
	StartedAt  *time.Time // §18.2 claim→terminal 측정
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// step0003 (PR 18) raw DDL 계약 — 모델 태그로 표현 불가한 가드는 그 스텝이 소유한다:
//
//	active_flag TINYINT GENERATED ALWAYS AS
//	  (CASE WHEN status IN ('succeeded','failed','timed_out','cancelled') THEN NULL ELSE 1 END) STORED
//	UNIQUE KEY uq_provider_task_resource_active (resource_uid, active_flag)
//
// 종단 행은 NULL → 유니크에서 제외(MySQL 부분 인덱스 미지원의 표준 우회, A5).
// CASE 목록 4종이 provider_task.Status 종단 집합 그 자체다(r2 A — §6 T24와 동치).
// 빈 resource_uid 태스크는 유니크 우회 수단이 되므로 Submit이 ”를 하드 거부한다(T-5) —
// 컬럼 계약은 not null(빈 문자열 허용, A3), 엔진 계약이 상류에서 봉쇄한다.
func (ProviderTask) TableName() string { return "provider_task" }

// TaskAttempt is §13.1 "one row per execution attempt (worker, timing, error)".
type TaskAttempt struct { // §13.1 "one row per execution attempt (worker, timing, error)"
	ID           uint
	TaskID       uint   `gorm:"not null;index:idx_attempt_task_no,unique"`
	AttemptNo    int    `gorm:"index:idx_attempt_task_no,unique"` // (task, no) 1회성 보장
	WorkerID     string `gorm:"size:128"`
	HandleRef    string `gorm:"size:255"` // provider-네이티브 핸들(예: UPID, §14.3)
	StartedAt    time.Time
	FinishedAt   *time.Time
	ErrorCode    string `gorm:"size:128"`
	ErrorMessage string `gorm:"type:text"`
	CreatedAt    time.Time
}

func (TaskAttempt) TableName() string { return "task_attempt" }

// TaskEvent is §13.1 "append-only state/event log, written transactionally" (D5).
type TaskEvent struct { // §13.1 "append-only state/event log, written transactionally" (D5)
	ID        uint64
	TaskID    uint `gorm:"not null;index"`
	AttemptNo int
	Type      string           `gorm:"size:64;not null"` // tasks/state.go 어휘(A4)
	Actor     string           `gorm:"size:128"`         // 승인자 등
	DataJSON  contract.JSONMap `gorm:"serializer:json;type:text"`
	At        time.Time        `gorm:"index"`
}

func (TaskEvent) TableName() string { return "task_event" }
