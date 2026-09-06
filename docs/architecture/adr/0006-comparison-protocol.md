# ADR-0006: Legacy/V2 Comparison Protocol

## Status

Accepted (2026-09-06, elevates spec r2 §15; cross-references §19 (shadow-read definition) and §20 (Phase 2 and Phase 6 gates). Implemented in Phase 0 contract only — the comparison tooling lands in Phase 2 per §20).

## Context

- r1's Phase 2 said "compare legacy and V2 inventory results" and the r1 frontend section said "shadow-read: compare legacy responses with new projections". Both were unverifiable as written (§15): the legacy side has no stored inventory — the cluster detail path is on-demand with a process cache, and the database stores only connection info. There was no capture method, no field mapping, no pass criterion, and no concurrency guarantee. A comparison without those four components can neither pass nor fail; it can only be asserted.

## Decision

Legacy/V2 comparison is a protocol with four fixed components (§15):

1. **Snapshot capture** (§15.1): a CLI subcommand of the same binary, admin-only, never an HTTP route. The legacy side is captured by calling the same service methods the v1 API handlers call, with the process cache bypassed; the V2 side is the completed shadow sync run's published generation, read back through V2 projection queries, not internals. Pairing rule: v2 sync completes, legacy capture runs immediately after, the pair's timestamps are recorded; the capture delta must be within 60 seconds for count/identity comparisons to be evaluated at all. An over-running capture re-pairs once with sections captured serially and volatile sections dropped from that pair; a second over-run is a BLOCKER report, not an infinite re-capture loop. Snapshots are dated artifacts retained for the Phase 2 duration — not test fixtures.
2. **Field mapping** (§15.2): the authoritative mapping table lives next to the normalizer (one file per resource family, reviewed in the family's Phase 2 PR). Every field the legacy API response serializes appears in the table — mapped, or explicitly marked dropped with a reason. Unmapped legacy fields are protocol bugs, not noise.
3. **Mismatch classification** (§15.3): BLOCKER (identity sets differ; counts differ beyond tolerance within the pairing window; a non-volatile normalized field differs), VOLATILE (timestamps, ages, restart counts, ordering — allowed to differ unconditionally), DRIFT (the window was exceeded — not evaluated; triggers a paired re-capture), ABSENT (dropped by the mapping table — logged, not failed). Third-party mutations inside the capture window land in DRIFT: the honest answer to "no concurrency guarantee exists" is to bound and re-measure, not to pretend one.
4. **Pass and abort criteria** (§15.4): pass = zero BLOCKER mismatches, every VOLATILE difference logged into the report, and three consecutive passing paired runs on different days. Abort = a BLOCKER on two consecutive paired runs after one DRIFT re-capture — stop shadow-sync development on that cluster, fix the normalizer or the mapping table, restart the three-run clock. Identity conflicts inside the sync run itself abort immediately.

The same protocol, with a per-family mapping table, is the shadow-read definition of Milestone 2 cutover (§19); Phase 2's exit gate cites the stored comparison reports as evidence (§20).

## Rationale

- Identity is the hard property: a resource existing on one side only invalidates every other comparison, which is why BLOCKER exists as a class and why identity_conflict aborts immediately (§15.3, §15.4).
- Tolerance windows exist because the two sides are captured at different instants; pretending simultaneity would fake precision and make every failure arguable.
- Stored dated artifacts make the gate checkable — §20's preamble rule: an artifact cited by a gate is stored and referenced, not summarized from memory.

## Alternatives considered

- Ad-hoc diff without pairing (r1's shape) — rejected: unverifiable. Differences could always be attributed to timing, so nothing could ever deterministically pass or fail (§15 preamble).
- Assuming concurrency guarantees during capture — rejected: none exists; the DRIFT classification is the explicit, bounded answer (§15.3).
- Test fixtures as comparison sources — rejected: §15.1 retains live captured artifacts; fixtures would validate the fixture author, not the migration.

## Consequences

Positive: shadow-read becomes a falsifiable gate for the Phase 2 vertical slice and for every Milestone 2 cutover family; normalizer and mapping bugs surface as BLOCKER reports instead of review impressions.

Side effects: every resource family carries a maintained mapping file next to its normalizer — a reviewed artifact with a coverage rule; comparison runs spend capture time on real clusters and re-pair on DRIFT, so gate completion is measured in days (three runs on different days), not hours.

Revert cost: the tooling is additive; abandoning the protocol for a family simply blocks that family's cutover gate — which is the intended failure mode.

## Enforcement

- §23.4 N12: comparison BLOCKER on identity mismatch → the Phase 2 gate refuses to pass (runs in CI; a green suite without N1–N12 is not green).
- §20 Phase 2 exit gate: §15.4 pass (three consecutive clean paired runs), identity-conflict count zero, and the gate cites the stored comparison reports.
- §19.1/§20 Phase 6: per-family §15.4 pass is a precondition of cutover, together with a completed restore rehearsal and an observation period with zero legacy-path traffic before drops.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §15.1–15.4, §19 (migration strategy; shadow-read definition), §20 (Phase 2 and Phase 6 gates), §23.4 (N12)
- ADR-0002 (the V2 projection this protocol compares against), ADR-0001 (the Phase 6 decomposition gated by it)
