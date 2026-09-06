# ADR-0001: Modular Monolith with an In-Process Adapter Boundary

## Status

Accepted (2026-09-06, elevates spec r2 §1, §6, §3, §11.2/§12; implemented in Phase 0 contract terms only — decomposition work belongs to Phase 6, conditional, Milestone 2).

## Context

Measured facts that frame the decision:

- The deployment is one replica: docker-compose with a single API process (§5.1, §2.8). The multi-replica concerns r1 warned about are hypothetical today; the crash-recovery gap is the present defect.
- The adversarial review (§2) found real structural defects — a God-object service (§2.5), a monolithic router and permission table (§2.6), provider selection implemented with switches (§2.7) — but none of them is cured by splitting the process.
- The domain disposition matrix (§3.2) gives every persisted model exactly one disposition; most of the product (DNS, certificates, database workbench, monitoring, CI/CD, notify) REMAINs V1 and is not absorbed by V2 in Milestone 1.
- r1 diagnosed the God object correctly but reacted with an oversized plan (24 new tables against a single replica, providers with zero existing code scheduled first); r2's correction is scope, not process topology (§1).

## Decision

- Keep a modular monolith through the first transformation stages (§1). The V2 control-plane core (§6) is a single process: provider type registry, connections and contexts, inventory service, orchestrator, durable task engine, secret broker, audit, observability.
- The core↔adapter boundary is enforced structurally inside the process: provider support enters only through the adapter interfaces of §11 (BaseAdapter, Discoverer, OperationExecutor, TaskPoller, TaskCanceller), registered via the provider type registry (§7.1 — descriptors are code, registered at compile time; no provider-type table exists in M1).
- The core never switches on provider product names; adapters never write control-plane tables directly (§6 architectural rules 1–2).
- God-object decomposition is a Phase 6, Milestone 2, conditional decision gated on per-family comparison passes and a restore rehearsal (§19.1, §20) — not an open-ended refactor lane.
- Built-in adapters run in process; a third-party adapter runtime exists only behind its own ADR (§6 rule 9, §11.2, §12.2).

## Rationale

- A single maintainer delivering independently shippable milestones (§5) gains nothing from distributed-systems overhead; r2 cut the plan to what one person can deliver and stop at safely (§1).
- The measured defects (§2.5–§2.7) are coupling defects. Package boundaries and interface contracts cure them at far lower risk than a process split, which would add deployment, serialization, and token-forwarding problems the repository has no tooling for.
- The disposition matrix (§3.2) makes "remain V1" a first-class answer, which keeps the monolith's blast radius explicit and reviewable per PR (§3.4).

## Alternatives considered

- Microservice decomposition now — rejected. It is an overreaction to the §2.5/§2.6 critique: it moves coupling across a network boundary instead of removing it, against a one-replica, one-maintainer reality (§5.1, §5.3).
- Out-of-process adapter runtime in M1 — rejected. r1's gRPC/mTLS runtime is deferred with the agent protocol (§11.2); its re-entry trigger is a real third party wanting to ship an adapter, and the design work is an ADR at that time, not before.
- Absorbing REMAIN domains into V2 while restructuring — rejected by §3.2/§3.4; silently broadening V2 into a REMAIN domain is a reviewable violation.

## Consequences

Positive: the boundary is testable in-process (the fake adapter is a first-class test double, §11.1, §23 contract tier); every milestone stays independently shippable (§5.4); V2 increments remain additive.

Side effects: boundary discipline now depends on static checks and review rather than a process wall — which is what makes the Enforcement section below load-bearing. Any future decomposition must first pass the Phase 6 gate, family by family, under the comparison protocol (ADR-0006).

Revert cost: adopting the monolith is the null transformation; the expensive path was rejected, not chosen.

## Enforcement

- §22 #10: third-party adapters inside the trusted API process are prohibited — enforced by the deferred out-of-process runtime ADR (§11.2).
- §22 #18: a PR that broadens V2 into a §3 REMAIN domain is rejected on review — enforced by the §3 boundary and the reviewer checklist.
- arch-boundary CI (`scripts/check-arch-boundary.sh`, the `arch-boundary` job plus the standing `arch-boundary-canary` job in `.github/workflows/v2-ci.yml`): R1 forbids core-package imports of provider product packages; R2 greps core sources for provider product identifiers; R3 enforces vocabulary-package import purity. The canary plants R1/R2 violations in a scratch copy and must fail with attributed output (§4.10: a gate that cannot fail is decoration; §20 Phase 0 gate 2).
- Scope qualifier (r2 handoff): the current `CORE_PACKAGES` list covers `internal/domain/dnsserver` only. Extending it to the new `internal/infra` core packages is not a one-line constant change — R2 is a whole-source grep for product identifiers and would fire on the contract package's legitimate type vocabulary — so the R2 rule must be redesigned around imports/type references first. That redesign is an explicit handoff and is not part of this ADR's acceptance.
- §22's closing note that the "Enforced by" column is "cited in review templates": see ADR-0003's Enforcement section for the repository-level determination of that clause.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §1, §6 (target architecture, architectural rules), §3 (disposition boundary), §11.2/§12 (deferred runtime and agents), §19.1, §20 (Phase 6 gate), §22 (#10, #18)
- docs/architecture/multi-infrastructure-control-plane-epic.md §4 — background only
- scripts/check-arch-boundary.sh, .github/workflows/v2-ci.yml — the enforcing CI assets
