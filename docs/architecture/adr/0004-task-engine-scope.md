# ADR-0004: Task Engine Scope — Lightweight Durable Engine, Replica Mechanics Deferred

## Status

Accepted (2026-09-06, elevates spec r2 §13, §3.3, §13.6 with §5.5 triggers; implemented in Phase 0 contract only — the engine itself (poller, reaper, approval columns, idempotency) lands in Phase 1 with the fake adapter cycle).

## Context

- Measured defect: current asynchronous operations create records and then execute work in process-local goroutines; a process crash leaves records stuck in running state with no recovery (§2.8). The deployment is a single instance; the multi-replica scheduler collision r1 warned about is hypothetical, the crash-recovery gap is present today.
- r1 specified lease + fencing + CAS + heartbeat + four timeout classes + outbox + inbox + idempotency + lock tables: a multi-replica distributed system. The deployment is one replica (§13).
- r1's absorption claim was self-contradictory: it said "execute every infrastructure mutation through the durable task engine" and prohibited process-local mutations, while the product's actual mutation families — DNS batch changes, SSL apply/renew, ACME, database backup, ops scripts/exec/schedule, CI/CD pipelines — were never scheduled for absorption (§3.3).

## Decision

- V2-originated mutations execute through a lightweight durable task engine (§13.1) built on exactly three tables: `provider_task` (unit of work + lease column + approval columns), `task_attempt` (one row per execution attempt), `task_event` (append-only state/event log, written transactionally with state changes).
- Workers are in-process goroutines claiming via a poller (default 2s interval, configurable) using a single-statement atomic claim (§13.2). A stale-lease reaper re-queues (attempts remain) or fails with a lease-expired error (attempts exhausted). Crash recovery bound: lease + grace + poll interval — the measurable fix for §2.8's orphaned-running defect.
- Concurrency and idempotency (§13.4): optimistic `Version` column on every write; unique `idempotency_key` where present, duplicate submits return the existing task with a replay header and never a second execution; per-resource serialization via an `active_flag` generated column and unique index — two non-terminal mutations of one resource cannot coexist; two timeout classes (provider call per attempt, task deadline overall); cancellation via status flag, persisted either way.
- Approval reuses the existing OpsJob semantics — `ApprovalStatus` (default `not_required`) + `Approver` on the task row (§13.3). No parallel approval model, no new approval tables.
- Absorption is an explicit list, not a slogan (§3.3): Milestone 1 absorbs Kubernetes workload restart (the Phase 3 proof) and future V2 provider operations — nothing else. The listed V1 families keep their current execution paths until their Milestone 2 convergence decision, at which point each either migrates with its own shadow run or is recorded permanently exempt with a reason.
- Deferred replica mechanics with written triggers (§13.6, §5.5): fencing tokens and heartbeats → a second API replica is actually deployed; outbox/inbox → a second consumer of task events exists; four timeout classes → workflow-style tasks arrive. Each trigger is falsifiable.

## Rationale

- The engine's first job is crash recovery for in-flight mutations — solvable single-writer with a lease column and a reaper (§13.2). Everything r1 added serves the replica case, and §5.5 shows that trigger has not fired.
- `task_event` committed in the same transaction achieves the outbox's goal because there are no external consumers (§13.2); the moment one exists, the ledger row is revisited — the deferral is conditional, not permanent.
- The explicit absorption list resolves r1's contradiction honestly: the prohibition ("process-local goroutines as durable infrastructure jobs") applies to new V2 code, and the list is the seed of the exemption record (§3.3).

## Alternatives considered

- r1's full machinery (fencing + heartbeat + outbox + inbox + four timeout classes) — rejected: designed for a second replica that does not exist; every piece re-enters only when its §5.5/§13.6 trigger fires.
- External broker/queue — rejected: message brokers are an explicit non-goal (§1) with a §7.7/§13.6 ledger entry and re-entry trigger.
- Absorbing the V1 mutation families in M1 — rejected by §3.3: convergence is a Milestone 2 decision per family, either migration with a shadow run or recorded exemption.

## Consequences

Positive: the orphaned-running defect gets a bounded, testable fix; idempotent replay and per-resource uniqueness are structural rather than conventions; approval UX reuses semantics operators already know (§13.3).

Side effects: process-local goroutines as durable infrastructure jobs are prohibited for new V2 code (§22 #7) — V1 lanes are exempt until convergence, and the §3.3 exemption list must be maintained rather than ignored. Polling cycles are attempts, visible in `task_attempt`/`task_event`, not a distinct state (§13.5).

Revert cost: the engine lands in Phase 1 additively; cost attaches when the first production task rows exist, at which point the §4.5-style discipline (ADR-0005) applies to any schema touch.

## Enforcement

- §22 #7: process-local goroutines as durable infrastructure jobs — applies to new V2 code only (§3.3); enforced by engine tests.
- §23 engine tier (against a real MySQL): claim races under `-race` with parallel claims, stale-lease reaper recovery, idempotency replay, resource-uniqueness rejection, crash injection (SIGKILL worker between claim and commit) — the engine+secrets scenario list must be green under `-race`; coverage alone is insufficient (§23.3).
- §23.4 negative tests: N6 (duplicate idempotency key → same task returned, single execution asserted at the provider double, not the log), N7 (stale lease after simulated crash → reaper re-queues, no lost task), N11 (second mutation on same resource → fast fail).
- §20 Phase 1 exit gate: crash-recovery test green under `-race`; duplicate key returns the same task; per-resource uniqueness blocks a second active mutation; fake adapter completes a full plan→approve→execute→audit cycle in tests.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §13.1–13.6, §3.3 (absorption list), §5.5 (deferral triggers), §2.8 (the measured defect), §20 (Phase 1), §22 (#7), §23 (engine tier, N6/N7/N11)
- ADR-0001 (the monolith whose single process hosts this engine), ADR-0002 (the adapter interfaces it drives)
