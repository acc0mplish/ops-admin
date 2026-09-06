# ADR-0005: Secret Envelope v2 — Format, Key Set, Classification, and Re-encryption Order

## Status

Accepted (2026-09-06, elevates spec r2 §4.1–4.5). This ADR is the contractualization of an already-delivered design: the envelope, field registry, classifier, migration commands, and the executed key-rotation run all landed in Phase -1 (commits listed under References). Later changes to the envelope format, the classification rule, or the step order require a new ADR — not a code PR.

Literal policy: this ADR deliberately renders no example envelope value, no example key identifier, no nonce, and no base64 body. The normative grammar is spec §4.2; reproducing secret-shaped literals in documentation mis-trains the secret scanner and its canary (§4.10).

## Context

- §4.1 measured the complete secret-field inventory: 14 live rows (4 legacy-envelope class, 10 plaintext class; one r1 phantom row was removed on measurement). The registry is the contract: a field not listed is not a secret; a listed field must satisfy §4.2–4.5.
- Two measured defects made the change mandatory: a decryption path that treated a stored value as plaintext when decryption errored, and a hard-coded development fallback key inside the secret utility that a production start could silently run with (§4.1, §2.9/§2.10).
- Legacy values carried no format self-description, so key rotation across the four affected domains (DNS accounts, SSL private keys, ACME, schedule secrets) could only identify the right key by trial decryption — a fail-closed outage risk (§4.4).

## Decision

- Envelope format v2 (§4.2): a self-describing three-part value — a version tag identifying the v2 format, the identifier of the master key that encrypted the body, and a base64url body carrying the concatenated nonce and ciphertext. Readers select the key by identifier, never by trial.
- Key set (§4.2): supplied by the ordered master-keys environment variable — current key first, previously retired keys after; the writer always uses the current key; the legacy key enters the set under a reserved identifier so dual-key readers span the migration window without format probing. Key material is the raw seed string, processed exactly as the pre-existing derivation.
- Triple detection rule (§4.3): the field registry decides first, the value second — NOT_SECRET, EMPTY, V2, LEGACY, PLAINTEXT, UNKNOWN in that order. UNKNOWN never falls through to plaintext: it halts migration and is reported as (model, row, field) for manual repair. The classifier is a pure function of (registry, value) and is shared, as one implementation, by the migration tool and the runtime decrypt path so they cannot disagree. Mixed-declaration columns take a caller-supplied per-value declared-secret gate; the registry alone never decides for them.
- Order contract (§4.4), mandatory and unmergeable: (1) ship writer + dual-key reader — no data rewritten, no key removed; (2) migrate data row by row in registry order — classify, read via an accepted path, write v2 under the current key, verify by byte-compare decrypt, checkpoint for resumability, column backup taken before first write; (3) verify — a gate, not a vibe: every registry field is V2 with zero LEGACY/PLAINTEXT/UNKNOWN, plus sampled spot-decrypts and a functional smoke; (4) retire the legacy path in a separate PR — plaintext-tolerant fallback deleted, dev fallback key removed from non-dev paths, legacy key removed from the key set, reader becomes v2-only with hard error + audit on anything else; (5) destroy the column backup after an observation period.
- Rollback (§4.5) is per stage: revert the deployment at stage 1 (v2 read support is kept in perpetuity); restore the pre-migration column backup at stage 2; at stage 4, re-add the legacy key to the key set and revert the retirement PR — v2 data is unaffected. Backup destruction (stage 5) happens only after one clean release cycle past stage 4.

## Rationale

- Self-description removes trial decryption — the property whose absence made rotation a fail-closed risk (§4.4). Rotation cannot cause a domain outage because dual-key reading spans the entire window in which ciphertext is mixed.
- The classification rule's strictness is the structural fix for the plaintext fallback: ambiguity is an incident, not a parsing strategy (§4.3).
- Verification before retirement is what makes legacy-key removal safe; deleting the fallback earlier would convert every not-yet-migrated field into an outage (§4.4 invariant).

## Alternatives considered

- Tolerating plaintext on decrypt error (the existing fallback) — rejected: it is the measured §2.9/§4.1 defect; after stage 4 the reader is v2-only and anything else is a hard error plus audit event.
- Keyless identification (trial decryption across configured keys) — rejected: it is the failure mode this envelope exists to remove.
- External vault backend in M1 — rejected: an explicit non-goal (§1); `SecretRef.Backend` (§7.5) reserves the seam while the backend stays internal.
- Migrating data before shipping the dual-key reader — rejected by the P-class ordering rule (§4.3 amendment): the gate is order-dependent, and rewriting columns ahead of reader support breaks plaintext readers.

## Consequences

Positive: rotation is self-describing and per-stage rollback-safe; the classifier's per-field counts are machine-checkable gate artifacts (G-1, G-2) rather than assertions.

Side effects: every secret-bearing field added later must join the §4.1 registry and ship together with its writer conversion — the registry is closed and reviewed by diff. Documentation and code outside the secret chain must not render secret-shaped literals, or they become scanner findings (this ADR's own rule).

Revert cost: bounded per §4.5 stage. The one irreversible step — backup destruction — is gated on a clean release cycle after retirement.

## Enforcement

- §20 Phase -1 gates: G-1 (classifier report: zero PLAINTEXT, zero LEGACY, zero UNKNOWN across all 14 registry rows, post-stage-3 artifact), G-2 (rotation report: migrated == verified == total per field), G-5 (negative tests green: production startup without a master key fails — §23.4 N4; undecryptable value → hard error, no plaintext passthrough — N5).
- CI baseline (§4.10, `.github/workflows/v2-ci.yml`): the `secret-scan` job including its canary (a planted fake secret must be reported, and a canary that does not fire fails CI) plus the untracked-config history guard; the `migration-test` job asserting post-migration field classification.
- §23.4: N4 and N5 run in CI; a green suite without N1–N12 is not green.
- The full git history is deliberately not scanned (config files existed in history before being untracked); the guard instead asserts they stay untracked and git-ignored.

## References

- docs/architecture/multi-infrastructure-control-plane-v2.md §4.1–4.5, §4.10, §7.5 (SecretRef), §20 (G-1–G-5), §23.4 (N4, N5)
- Phase -1 delivered implementation: T1 envelope/registry/classifier (d9aa334, f3fd302, 31f6b33), T1 fixes (d362699, 0344ea8), T2 config untracking (b43aa6d), Step 2/3 commands and G-5 startup guard (bba74f1), re-encryption command coverage (9b6cba8), secret-scan/arch-boundary scripts (06c23be, 362581c), executed key-rotation dev run (0a17a77)
