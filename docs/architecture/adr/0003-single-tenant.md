# ADR-0003: Single-Tenant by Explicit Decision

## Status

Accepted (2026-09-06, elevates spec r2 §7.6 and §1 explicit non-goals; implemented in Phase 0 contract only — this is a standing design invariant, not a work item).

## Context

- Measured: zero `TenantID` fields across every current model (§7.6). There is no tenant concept anywhere in the running system.
- r1 nonetheless required `ProviderTask.TenantID`, "tenant context in token", and tenant-bearing audit records — none of which had a defining Phase, entity, or ADR (§7.6).
- The V2 policy engine evaluates a resource-scoped policy input on top of role grants (§10.3); r1's design left the tenant dimension of that input undefined — precisely how implicit tenancy would have leaked into every table.

## Decision

- Single-tenant is a design invariant, not a TODO (§6 rule 10).
- There is no Tenant entity and no tenant claim in tokens (§7.6).
- The policy input's `tenant` field is the constant `default`, set at the API boundary (§7.6).
- Audit records carry no tenant column in M1 (§7.6, §18.1).
- A provider project/account/subscription is provider-native scope (`ProviderContext`, §7.3) and is never equated with a platform tenant (§22 #13). K3s-like distributions do not become tenants either (§22 #14 governs the adjacent confusion).
- Multi-tenancy re-entry requires its own ADR touching identity, tokens, and every scoped table (§1 non-goals). The trigger is falsifiable: a second organization actually using the system (§5.5, §7.7). "We might need it someday" does not satisfy it.

## Rationale

- Retrofitting tenancy implicitly is the worst of both worlds: schema churn across every table with no requirement behind it — the §7.6 measurement (zero tenant fields) is the evidence that no requirement exists.
- Keeping the constant inside the policy input preserves the seam where a future tenant value would bind, without inventing a second authorization model or JWT claim surgery today (§1 non-goals).
- An explicitly stated non-goal prevents scope creep better than an omitted one (§1).

## Alternatives considered

- r1's tenant machinery (`ProviderTask.TenantID`, token tenant claims, tenant audit columns) — rejected: no measured need, no defining design, and it violates §5.5's falsifiability standard for deferred scope.
- "Decide later" / leave the policy-input dimension undefined — rejected: an undecided dimension is how tenancy retrofits itself; §1 states the non-goal precisely to close this door.
- Treating provider-native scopes (accounts, projects, subscriptions) as tenants — rejected by §22 #13: they are `ProviderContext` rows, a different concept with different lifecycle.

## Consequences

Positive: all 13 M1 tables are tenant-free by construction; authorization stays role grants + resource-scoped policy; audit stays compact (§18.1); no token-format churn accompanies any V2 milestone.

Side effects: if a second organization ever arrives, the change is deliberately expensive — a dedicated ADR spanning identity, tokens, and every scoped table (§1). That cost is the point: it cannot be drifted into.

Revert cost: none now; re-entry cost is intentionally high and front-loaded into the required ADR.

## Enforcement

- §22 #13: equating provider project/account with platform tenant is a prohibited shortcut ("Enforced by: §7.6 single-tenant constant").
- §22 #16: creating any §7.7 deferred table without its trigger fired — `ProviderTenantBinding` is one of those ledger rows, and its trigger ("multi-tenancy ADR accepted") cannot fire quietly.
- Review checklist (recorded here, as the operative form):
  1. no new V2 migration adds a tenant column, including audit;
  2. no token parsing introduces a tenant claim;
  3. the policy input `tenant` field remains the constant, set at the API boundary;
  4. `ProviderContext` rows are never presented, queried, or filtered as tenant boundaries.
- Determination (r2): §22's closing note says the "Enforced by" column is "cited in review templates". This repository contains no review-template mechanism (no PR template or equivalent exists in `.github/`), so that clause has no object to attach to in this repo today. The checklist above is therefore recorded in this ADR as its operative substitute; when a review template is introduced, it should cite this section.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §7.6 (tenancy: the single-tenant constant), §1 (explicit non-goals), §7.7 (deferred ledger: ProviderTenantBinding), §5.5 (deferral triggers), §10.3 (policy input), §22 (#13, #14, #16)
