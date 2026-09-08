package compose

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
	"ops-admin/backend/internal/infra/model"
	"ops-admin/backend/internal/infra/registry"
	"ops-admin/backend/internal/infra/secrets"
)

// healthSweepInterval — §18.2 provider_health 계기 주기(가정 A6: 상수 5분).
// 주기 전 커넥션 probe는 4개 클라우드로 실요청이므로 R2 완화상 상수다 —
// 레이트리밋 신호는 provider_rate_limit_total 계기가 자동 관측한다.
const healthSweepInterval = 5 * time.Minute

// healthPurpose — 스윕이 해석하는 자재 목적(가정 A6 — 실재 바인딩이 있는
// 유일 목적; monitoring은 바인딩 부재로 전커넥션 unhealthy가 된다). sync의
// ConnectionView 조립(resolveConnectionView)과 같은 §7.4 어휘다.
const healthPurpose = "inventory"

// HealthSweeper is the §18.2 provider_health gauge circuit (P6): 주기마다
// provider_connection 행 순회 → "inventory" 목적 자재 해석 → registry
// 어댑터 Health → SetHealth(uid, healthy). 바인딩 부재·어댑터 미등록·DB
// 질의 실패는 unhealthy(0) 적립이 곧 계기 형상(계획 claim 10). health
// 성공 시 provider_connection.last_health_at을 갱신한다 — 이 회로가 그
// 컬럼의 기록 주체가 된다(api/v2 읽기가 증분 소비). 프로덕션 동작(동기화·
// 실행·재큐)은 무관한 관찰 전용 회로다.
type HealthSweeper struct {
	db       *gorm.DB
	registry *registry.Registry
	broker   *secrets.Broker
	counters *metrics.Counters

	mu       sync.Mutex
	observed []string // 마지막 정상 sweep이 관측한 UID — DB 실패 회의 0 적립 대상

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewHealthSweeper assembles the sweeper over the stack's shared
// dependencies — the same registry the adapters registered into (등록 정확
// 1회 계약) and the same counters every other §18.2 family accrues to.
func NewHealthSweeper(db *gorm.DB, reg *registry.Registry, broker *secrets.Broker, counters *metrics.Counters) *HealthSweeper {
	return &HealthSweeper{
		db:       db,
		registry: reg,
		broker:   broker,
		counters: counters,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start runs the sweep loop until Stop. The first sweep waits one full
// interval (ticker semantics — 부트 직후 실클라우드에 probe를 쏘지 않는
// 것이 R2 완화). 테스트는 Start 없이 SweepOnce를 직접 드라이브한다.
func (s *HealthSweeper) Start(ctx context.Context) {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(healthSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				if err := s.SweepOnce(ctx); err != nil {
					log.Printf("compose: health sweep failed: %v", err)
				}
			}
		}
	}()
}

// Stop cancels the loop and waits for the goroutine to exit. nil 수신자는
// no-op다 — 엔진 레인 부재(R11) 부트에서도 호출부가 가드 없이 잇는다.
// Start 이전의 Stop은 done이 닫히지 않으므로 호출해서는 안 된다(engine.Stop
// 과 같은 계약 — 기동 순서는 호출부가 소유한다).
func (s *HealthSweeper) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.done
	})
}

// SweepOnce executes one health sweep. 개별 커넥션의 실패 단계는 전부 0
// 적립으로 소화된다(R2 — 프로세스 수명 회로에서 한 커넥션 실패가 sweep 전체를
// 죽이지 않는다); SweepOnce의 error는 전체 행 로드 실패와 last_health_at
// 기록 실패의 join뿐이다.
func (s *HealthSweeper) SweepOnce(ctx context.Context) error {
	var conns []model.ProviderConnection
	if err := s.db.WithContext(ctx).Find(&conns).Error; err != nil {
		s.markObservedUnhealthy()
		return fmt.Errorf("compose: health sweep connection load: %w", err)
	}

	uids := make([]string, 0, len(conns))
	var join []error
	for _, conn := range conns {
		uids = append(uids, conn.UID)
		healthy := s.probe(ctx, conn)
		s.counters.SetHealth(conn.UID, healthy)
		if healthy {
			if err := s.markLastHealthAt(ctx, conn.UID); err != nil {
				join = append(join, err)
			}
		}
	}
	s.setObserved(uids)
	return errors.Join(join...)
}

// probe resolves the sweep's single provider leg: "inventory" 자재 해석 →
// 등록 어댑터 → Health. healthy는 false가 기본 — 해석 실패(바인딩 부재)와
// 어댑터 미등록은 gauge 0 적립으로 소화된다(claim 10). ConnectionView는
// resolveConnectionView와 동일 형상이다(UID 미채움 — 발견 경로 계약 유지).
func (s *HealthSweeper) probe(ctx context.Context, conn model.ProviderConnection) bool {
	resolved, err := s.broker.Resolve(ctx, conn.UID, healthPurpose)
	if err != nil {
		return false
	}
	_, adapter, ok := s.registry.ProviderType(conn.ProviderType)
	if !ok {
		return false
	}
	view := contract.ConnectionView{
		ProviderType: conn.ProviderType,
		Endpoint:     conn.Endpoint,
		Config:       conn.ConfigJSON,
		Material:     map[string]string{healthPurpose: resolved.Value},
	}
	return adapter.Health(ctx, view).Healthy
}

// markLastHealthAt — health 성공 시 provider_connection.last_health_at을
// 지금으로 갱신한다. gauge는 이미 적립된 뒤 호출되므로 기록 실패는 측정값을
// 바꾸지 않고 SweepOnce의 error join에만 흘린다.
func (s *HealthSweeper) markLastHealthAt(ctx context.Context, uid string) error {
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&model.ProviderConnection{}).
		Where("uid = ?", uid).Update("last_health_at", now).Error; err != nil {
		return fmt.Errorf("compose: health sweep last_health_at update for %q: %w", uid, err)
	}
	return nil
}

// markObservedUnhealthy — DB 질의 실패 회(claim 10의 DB 실패 갈래): 마지막
// 정상 sweep이 관측한 커넥션 전부를 0으로 적립한다. 최초 실패(관측 이력 없음)는
// 적립 대상이 없다.
func (s *HealthSweeper) markObservedUnhealthy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, uid := range s.observed {
		s.counters.SetHealth(uid, false)
	}
}

func (s *HealthSweeper) setObserved(uids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observed = uids
}
