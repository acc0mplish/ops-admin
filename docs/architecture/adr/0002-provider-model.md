# ADR-0002: Provider Model — Descriptor, Registry, Capability, and the Connection/Context/Credential Split

## Status

Accepted (2026-09-06, elevates spec r2 §7.1–7.4, §10, §11; diagnosis from §2.2/§2.4/§2.7; §14 mapping matrix cross-referenced. Implemented in Phase 0 as contract code only — provider-type/capability/operation registries and the fake adapter (spec §21 #13); connection tables land in Phase 1, adapters in Phases 2–4).

## Context

- `AssetCloudAccount` assumed `provider + access key + secret key + regions` and cannot faithfully model Proxmox tokens and realms, vCenter sessions, CloudStack domain/account scoping, OpenStack application credentials, or Kubernetes kubeconfig/OIDC (§2.2). Measured reality: the same Aliyun/Tencent credentials are stored up to three times — cloud account (plaintext), finops account (plaintext), public DNS account (legacy envelope) — with different encryption postures per copy (§2.2, §4.1).
- `K8sCluster` mixes connection, secret, runtime, inventory, and monitoring in one record (§2.4).
- Provider selection is implemented with name switches, and Aliyun/Tencent discovery lives inside the service package (§2.7). Kubernetes-specific foreign keys already leak into shared domains (§2.3) — the per-provider column pattern scales badly.
- r1 designed a universal provider interface; the epic's §5 sketch of that shape is what r2 rejects.

## Decision

- Provider support is described by `ProviderTypeDescriptor` (§7.1): software support, not a configured endpoint; descriptors are code, registered at compile time; no provider-type table exists in M1. M1 types are kubernetes, aliyun, tencent, fake; proxmox/vcenter/cloudstack/openstack are reserved — descriptors land with their milestone, not before.
- One reachable control endpoint and transport = `ProviderConnection` (§7.2). One provider-native administrative scope = `ProviderContext` (§7.3, kinds: cluster, account, project, subscription; no self-referencing nesting in M1). A connection may bind several credentials for different purposes = `ProviderCredentialBinding` (§7.4). Secret material is only ever referenced through `SecretRef` (§7.5); `ConfigJSON` carries no secret values and `TLSProfile` only trust-material references. The triplicated cloud credential collapses into one SecretRef per distinct credential, with purpose bindings (inventory, billing).
- `Capability` (§10.1) and `OperationDefinition` (§10.2) are the source of truth for UI actions, authorization, risk, approval, auditing, retry, and redaction. Schemas are typed Go builders validated at registration; no new runtime dependencies in M1 (§10.2). Permission strings stay v1-namespace one-to-one (`domain:resource:action`, §10.3), so role grants carry over unchanged and the policy engine layers resource-scoped input on top.
- Adapters implement the §11 interface split — BaseAdapter, Discoverer, OperationExecutor, TaskPoller, TaskCanceller. Registry validation rules (§11): a declared capability must map to an implemented interface; operation definitions are immutable within a version; adapters never receive database handles; `fake` is a first-class adapter, not an afterthought.
- The core references adapter interfaces and capability names, never provider product names (§2.7 decision, §6 rule 1). The §14 provider mapping matrix is the standing cross-reference from provider families to phases and entry preconditions.

## Rationale

- The connection/context/binding split is exactly what makes the §2.2 list of unrepresentable provider shapes expressible without per-provider schema churn; §2.4's responsibility split falls out of the same three models.
- Capability-driven UI and API remove the per-provider static set problem (§22 #2) and give §3.4's reviewer the grounds to reject scope-broadening PRs mechanically.
- Keeping v1 permission strings (§10.3) makes v2 authorization a seed rather than a migration project.

## Alternatives considered

- A universal provider interface — rejected: the epic's §5 example shape is precisely the pattern §22 #3 prohibits; it degrades into dozens of mandatory methods that no single provider implements meaningfully.
- Switch/factory selection on provider name — rejected: that is the current regime (§2.7); it is the defect being fixed, not the fix.
- Extending `AssetCloudAccount` with per-provider nullable columns — rejected: §2.2 shows it already cannot model non-cloud providers, and §2.3 shows where per-provider columns lead.
- r1's aws/azure/gcp example list — rejected as an entry condition: those providers have no code and no stated need (§7.1).

## Consequences

Positive: new providers arrive as descriptor + adapter + capability/operation declarations with zero core schema change — the Phase 4 gate asserts the negative via arch-boundary (§20). FinOps reuse rides contracts (§10.1, `cost.read`) on the unified credential.

Side effects: every provider addition requires vocabulary review — the provider types, capabilities, and resource kinds are closed sets extended by reviewed diff, not by runtime registration (§7.1 reserved list). Descriptors are compile-time, so "trying a provider quickly" means a code PR by design (§22 #17).

Revert cost: the Phase 0 contract code is additive and unwired; revert cost begins to attach in Phase 1 when connection tables adopt real rows.

## Enforcement

- §22 #1: provider-specific foreign keys in shared domain tables — code review + §3 boundary (existing K8s FKs convert only at Milestone 2 cutover, §2.3).
- §22 #2: a new static menu/controller/service/table set for every provider — §3 + UI generated from capabilities.
- §22 #3: a universal provider interface with dozens of mandatory methods — §11 interface review. The interface list is closed: ConsoleBroker is M2+ behind the agent ADR (§11), and r1's EventSubscriber was removed until a watch-capable provider reopens the inbox decision (§7.7).
- arch-boundary R2: core sources must not reference provider product identifiers (aliyun/tencent et al.).
- The §11 registry validation rules are enforced in code at registration time by the Phase 0 registry (spec §21 #13); §22 #17 blocks scheduling code for a provider with no environment to test against.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §7.1–7.5, §10, §11, §14 (mapping matrix), §2.2/§2.3/§2.4/§2.7, §22 (#1, #2, #3, #17)
- docs/architecture/multi-infrastructure-control-plane-epic.md §5 — the rejected universal-interface shape
- backend/internal/infra/ (contract, registry, adapter/fake) — the Phase 0 code realization of this ADR
