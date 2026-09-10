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

// healthObservationKey — 어댑터 관측(HealthResult.Observation, ④ A′-1)의
// provider_connection.ConfigJSON 저장 키. 스위퍼는 프로바이더 무관 계층이라
// 관측 어휘(k8s certificates 등)를 모른다 — 단일 예약 키에 latest-wins로
// 기록하고 나머지 ConfigJSON 키는 보존한다. 조립 측 소비는
// inventory.BuildOverviewCertificates다.
const healthObservationKey = "health_observation"

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
	started  bool     // Start 경과 — Stop의 사전 호출 안전성(④리뷰 LOW)

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
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()
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
// Start 이전의 Stop도 안전(④리뷰 LOW — healthloop.go:92-100): started 플래그
// 미상태면 done을 닫아 즉시 반환한다.
func (s *HealthSweeper) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		s.stopOnce.Do(func() { close(s.done) })
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
		healthy, observation := s.probe(ctx, conn)
		s.counters.SetHealth(conn.UID, healthy)
		if healthy {
			if err := s.markLastHealthAt(ctx, conn.UID); err != nil {
				join = append(join, err)
			}
			if len(observation) > 0 {
				if err := s.markObservation(ctx, conn.UID, observation); err != nil {
					join = append(join, err)
				}
			}
		}
	}
	previous := s.swapObserved(uids)
	s.pruneVanished(previous, uids)
	return errors.Join(join...)
}

// swapObserved — 관측 집합을 현재 UIDs로 교체하고 이전 집합을 돌려준다.
// pruneVanished가 사라진 UID를 판정하는 기준값으로 쓴다.
func (s *HealthSweeper) swapObserved(uids []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.observed
	s.observed = uids
	return previous
}

// pruneVanished — 이전 관측 집합에는 있었으나 이번 sweep에 없는 커넥션(삭제된
// 행)의 health 시리즈를 소거한다(④리뷰 LOW — gauge.go:10). 소거는 렌더 라인
// 제거일 뿐 DB 접촉이 아니다.
func (s *HealthSweeper) pruneVanished(previous, current []string) {
	live := make(map[string]bool, len(current))
	for _, uid := range current {
		live[uid] = true
	}
	for _, uid := range previous {
		if !live[uid] {
			s.counters.RemoveHealth(uid)
		}
	}
}

// probe resolves the sweep's single provider leg: "inventory" 자재 해석 →
// 등록 어댑터 → Health. healthy는 false가 기본 — 해석 실패(바인딩 부재)와
// 어댑터 미등록은 gauge 0 적립으로 소화된다(claim 10). ConnectionView는
// resolveConnectionView와 동일 형상이다(UID 미채움 — 발견 경로 계약 유지).
// ④ A′-1(P1-G1): 어댑터가 실어 온 파생 관측도 함께 돌려준다 — 기록 주체는
// 이 스위퍼다(어댑터는 DB 핸들 무소유, arch rule 2).
func (s *HealthSweeper) probe(ctx context.Context, conn model.ProviderConnection) (bool, contract.JSONMap) {
	resolved, err := s.broker.Resolve(ctx, conn.UID, healthPurpose)
	if err != nil {
		return false, nil
	}
	_, adapter, ok := s.registry.ProviderType(conn.ProviderType)
	if !ok {
		return false, nil
	}
	view := contract.ConnectionView{
		ProviderType: conn.ProviderType,
		Endpoint:     conn.Endpoint,
		Config:       conn.ConfigJSON,
		Material:     map[string]string{healthPurpose: resolved.Value},
	}
	result := adapter.Health(ctx, view)
	return result.Healthy, result.Observation
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

// markObservation — healthy 회에서 어댑터가 실어 온 파생 관측(④ A′-1)을
// provider_connection.ConfigJSON에 병합 기록한다. markLastHealthAt와 같은
// 쓰기 패턴 — gauge는 이미 적립된 뒤 호출되므로 기록 실패는 측정값을 바꾸지
// 않고 SweepOnce의 error join에만 흘린다. 단일 예약 키(healthObservationKey)
// latest-wins이고 기존 ConfigJSON 키는 불변 사본을 만들어 보존한다.
func (s *HealthSweeper) markObservation(ctx context.Context, uid string, observation contract.JSONMap) error {
	var conn model.ProviderConnection
	if err := s.db.WithContext(ctx).Select("uid", "config_json").Where("uid = ?", uid).First(&conn).Error; err != nil {
		return fmt.Errorf("compose: health sweep observation load for %q: %w", uid, err)
	}
	config := make(contract.JSONMap, len(conn.ConfigJSON)+1)
	for key, value := range conn.ConfigJSON {
		config[key] = value
	}
	config[healthObservationKey] = observation
	// Struct + Select — config_json의 칼럼 serializer는 모델 필드 쓰기에만
	// 적용된다(map 값 Update는 JSON 텍스트를 이중 인코딩한다 — backfill.go 선례).
	patch := model.ProviderConnection{ConfigJSON: config}
	if err := s.db.WithContext(ctx).Model(&model.ProviderConnection{}).
		Where("uid = ?", uid).Select("config_json").Updates(patch).Error; err != nil {
		return fmt.Errorf("compose: health sweep observation update for %q: %w", uid, err)
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
