# Contract Patch Plan (hostile review, 2026-Q1) — STATUS: fully applied in v0.1.0

> Every blocker (B1–B7), high-priority fix (H1–H8), schema fix (S1–S16), policy fix
> (P1–P13), and fixture addition listed below has been implemented. This document is
> kept as the audit trail for the v0.1.0 contract hardening; the live contracts are
> `schemas/agent-provenance-v1.schema.json` + `.cue` and `policies/default.rego`, and
> the current security posture is documented in [`SECURITY_REVIEW.md`](./SECURITY_REVIEW.md).
> Notable v0.1.0 deltas beyond this plan: transparency-log evidence moved from a
> type blacklist to a metadata-only allowlist; v1 mcpServers/delegation gates fail
> closed when declared without verified context; delegation is validated per step.

Reviewed the contract pack against AGENTS.md, ARCHITECTURE.md, all `docs/*`, both schema files, `policies/default.rego`, and `tests/golden/README.md`. Hostile review below — minimal-patch oriented, no rewrites, no custom crypto, no replacement of in-toto/DSSE/Sigstore/OTel.

## 1. Critical blockers (must fix before implementation starts)

**B1. `extensions` is a privacy and trust bypass.**
`schemas/agent-provenance-v0.schema.json:84-89` and `agent-provenance-v0.cue:161-163` allow arbitrary URI-keyed objects with no constraints. The whole "no raw prompt", "no public raw content", "additionalProperties: false" promise is canceled the moment an agent writes raw prompts under `extensions["https://vendor.example/v1"]`. Neither schema, CUE, nor Rego inspect extension contents. This is the most exploitable hole in the design.
*Minimal patch:* require every extension value to be `{ uri, digest, mediaType, visibility }` with the same visibility/visibility-vs-raw rules as `evidence`. Inline raw blobs forbidden. Add a Rego rule that walks the `extensions` map and rejects any value with `visibility == "public"` or with `kind == "inline"`.

**B2. `verificationLevel` is self-asserted — no minimum-level enforcement in the default policy.**
`policies/default.rego` only checks `level in valid_levels`. A predicate signed in a malicious workflow can claim `evidence-grade` to legitimately use `local-only` evidence and pass verification in repos that "use agentattest". `context.json` (per `tests/golden/README.md`) has no `requiredLevel` field. The level escalation control only catches *upward* lying; *downward* lying is the realistic attack.
*Minimal patch:* add `context.requiredLevel` and a deny rule `level_below_required` that rejects when the predicate level is strictly below repo policy. Document `requiredLevel` in `tests/golden/README.md`. Add a fixture `invalid-level-downgrade`.

**B3. Subject set is checked as superset, not equal-set.**
`policies/default.rego:77-81` requires every *expected* subject to match *some* statement subject. Extra statement subjects pass silently. An attacker can stuff extra attacker-controlled subjects (e.g. a poisoned `repo-tree`) into the statement and still satisfy the verifier on the legitimate `patch.diff`. Any downstream consumer that re-reads the statement and trusts another subject is exploited.
*Minimal patch:* deny rule rejects when a statement subject is not in `context.subjects` (set equality). If you want to allow optional subjects, require them to appear in an explicit allowlist in `context`.

**B4. `humanApproval.state == "approved-before-merge"` is purely a self-claim.**
The high-assurance gate (`schema:252-260`, `cue:179-181`, `rego:131-135`) elevates `approved-before-merge` to a hard requirement, but neither schema, CUE, nor policy ever cross-checks it against verified review evidence (PR review API, branch protection, CODEOWNERS). A compromised workflow can simply set the field. This is "observability claim treated as attestation".
*Minimal patch:* require an `evidence[]` entry of `type: "approval"` whose `digest` covers a verified review payload (e.g. GitHub PR review JSON), and require `context.verifiedApproval` (boolean derived out-of-band by the verifier from the same digest) to be true. Without this, mark high-assurance as "claim, not proof" in `THREAT_MODEL.md`.

**B5. `witnesses[]` are ornaments — never re-checked.**
`witness.digest` is required, but no part of the system ever fetches the witness URI, re-hashes content, and re-validates the digest. High-assurance is currently satisfied by inventing one well-formed witness object. The CLAUDE.md "≥1 witness" is a string-length check, not a witness check.
*Minimal patch:* require `witness.type == "transparency-log"` to be backed by a Sigstore bundle that the verifier independently confirms (existing wheel — Rekor / Sigstore bundle verification), and require `type == "timestamp-authority"` to map to the verified envelope's RFC 3161 / TSA evidence. If a witness type cannot be machine-verified by v0, drop it from the enum (`independent-service`, `human-review-system` are both currently un-verifiable — remove until you have a verifier path).

**B6. Verifier identity checks are documented but not implemented.**
`docs/VERIFICATION_MODEL.md:97-98` lists `signer_identity_mismatch` and `builder_identity_mismatch` as failure codes. `policies/default.rego` never reads `context.verifiedSigner`, `context.verifiedBuilder`, or `context.verifiedWorkflow`. Policy-grade is documented as "verified non-local builder identity required", but the default Rego only checks that `environment.builder` *exists* in the predicate — i.e. self-claimed builder, the exact threat in `THREAT_MODEL.md` row 4.
*Minimal patch:* extend `context` with `verifiedSigner`, `verifiedBuilderId`, `verifiedWorkflowRef`, `verifiedIssuer`. Add deny rules that fail when those are empty at policy-grade or differ from `predicate.environment.builder.id` / `builder.workflowRef`. Block release of v0-beta task 8 acceptance until these rules exist with golden tests.

**B7. `evidence[].uri` accepts `data:` URIs and `repo.url` accepts credentials.**
`evidence.uri` is `string` 1..1024 with no scheme allowlist (`schema:752-757`, `cue:109`). A predicate can carry raw prompt content as `data:text/plain;base64,...` and still pass the privacy gate (the gate is keyed on `evidence.type` and `visibility`, not on URI scheme). Similarly, `repo.url` only requires `format: uri` / `^.+://.+` — `https://user:secret@host/repo` validates. Both are silent privacy leaks into signed/transparency-logged content.
*Minimal patch:* constrain `uri` patterns to `^(https?|artifact|s3|oci|spdx|cyclonedx):` (no `data:`, no `file:` for non-evidence-grade). Strip userinfo from `repo.url` (regex disallow `@` between scheme and host).

## 2. High-priority design fixes

**H1. Visibility gate is structural, not content-aware.**
Privacy enforcement keys exclusively off `visibility` and `evidence.type` enum values. It does not run any content scan. So `type: "trace"` with `storage: "transparency-log"` and `visibility: "public"` is allowed today, and a trace document can include raw prompts/messages. Mark this clearly in `PRIVACY_MODEL.md` ("we do not scan content"), and either (a) require traces destined for public storage to be wrapped in an OpenInference-redacted profile (reuse OTel privacy attributes), or (b) forbid `type: "trace"` from `storage: "transparency-log"` outright.

**H2. "isolated-runner" / "witnessed-runner" / `runnerClass: "isolated"` are untyped trust labels.**
There is no checkable definition. Any predicate can claim `executionType: "isolated-runner"`. The high-assurance gate consumes these strings and treats them as load-bearing. This is observability terminology dressed as attestation.
*Minimal patch:* require, at high-assurance, that `environment.builder.id` matches one of an out-of-band-allowlisted runner identifiers in `context.allowedIsolatedRunners` (verifier input, not predicate input). Without an allowlist match, fail closed. Don't try to define what "isolated" means in v0.

**H3. `runtime.traceIds` claims a trace exists; nothing binds the trace to the predicate.**
A trace ID is a 32-hex string trivially fabricated. The only binding to the actual trace document goes through `evidence[].type == "trace"` with a `digest`, but the schema does not require an `evidence` entry when `runtime` is populated.
*Minimal patch:* CUE-level constraint — if `runtime` is present, `evidence` must contain at least one entry of `type: "trace"` whose digest is the trace export hash. Add a fixture `invalid-trace-id-without-evidence`.

**H4. Cache stores verification *results*, not just refs.**
`ARCHITECTURE.md:15` says cache "indexes... verification results". This is dangerous even with "cache contents are never authoritative" — it normalizes treating cached pass/fail as a signal. Restrict the cache to *content-addressed digests*, statement paths, and last-seen timestamps. Don't cache booleans. Document this in `ARCHITECTURE.md` and `internal/cache` table.

**H5. Failure-code ordering is asserted in prose but not testable.**
CLAUDE.md mandates "reject unsupported predicate versions before policy evaluation" and "subject digest mismatches before approval-state checks". Rego `deny` rules are an unordered set; ordering is a Go-side concern. Add a Go-side fixture-driven test where a predicate has *both* `unsupported_predicate_version` *and* `subject_digest_mismatch`, and the verifier emits exactly the version code first. Without this test, Task 9's "stable failure codes" criterion is partially fictional.

**H6. Replay protection is under-specified.**
`THREAT_MODEL.md` row 2 promises binding of "repo URL, base commit, subject digests, PR/build context, run ID, signer identity, and timestamps". `policies/default.rego` checks repo URL, base commit, and subject digests. Run ID, signer identity, PR context, and timestamps are not enforced. Either:
- Pull the unimplemented items out of the threat-model promise, **or**
- Add them to the default Rego with `context.expectedRunId`, `context.expectedPullRequestNumber`, `context.freshnessWindowSeconds`, `context.now`. Add a `stale_attestation` rule keyed on `predicate.timestamps.finishedAt` vs `context.now` and `freshnessWindowSeconds`.

**H7. JSON Schema and CUE diverge on date-time, URL, and trace media-type.**
- `timestamps.startedAt/finishedAt`: JSON Schema uses `format: date-time` (informational); CUE uses raw `string`. CUE should add `=~` for RFC 3339.
- `repo.url`: JSON Schema `format: uri`; CUE `=~"^.+://.+"`. CUE accepts `urn:foo:bar`; JSON Schema validators differ. Pick one regex and mirror it.
- `runtime.traceFormat: "openinference-otel-json"` is a project-coined label, not an IANA media type. Either pin to existing OTLP media types (`application/x-protobuf`, `application/json` with profile parameter) or document the label is project-internal and add it to `EXISTING_WHEELS.md` rationale.

**H8. CLAUDE.md `additionalProperties: false` rule contradicts JSON Schema's interaction with conditional `then`.**
Verify with a fixture that a predicate with an unknown root key (e.g. `"foo": 1`) fails JSON Schema, *even when* one of the `if/then` branches matches. JSON Schema 2020-12 `additionalProperties` evaluates against the *parent's* `properties` only; conditional `then.properties` do not extend the known set. Confirm and add an explicit fixture `invalid-additional-properties-conditional`.

## 3. Schema-level fixes (`schemas/agent-provenance-v0.schema.json` + `.cue`)

| #   | Field                                                                 | Current                                  | Fix                                                                                                                                                                                        |
| --- | --------------------------------------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| S1  | `evidence[].uri` (`schema:752-757`)                                   | unconstrained 1..1024                    | scheme allowlist regex; forbid `data:`, `file:` (forbidden everywhere except evidence-grade `local-only`)                                                                                  |
| S2  | `repo.url` (`schema:286-290`)                                         | `format: uri`                            | regex disallowing `userinfo@`                                                                                                                                                              |
| S3  | `repo.branch` (`schema:299-303`)                                      | 1..255 free string                       | restrict to `^[A-Za-z0-9._/-]{1,255}$`; add note that branch text is signed                                                                                                                |
| S4  | `agent.declaredIdentity` (`schema:351-355`)                           | 1..256 free string                       | document — and forbid in privacy gate — that this must not be PII; pattern out `@` if you want a hard rule                                                                                 |
| S5  | `humanApproval.reviewerRefs[]` (`schema:563-571`)                     | 1..256 free string                       | require pattern e.g. `^[a-z]+:[a-z]+:[A-Za-z0-9._-]{1,200}$` (system:kind:id) so PII can't be smuggled                                                                                     |
| S6  | `runId` (`schema:24-29`)                                              | `^[A-Za-z0-9._:-]+$` 8..128              | tighten to ULID/UUID/CI-run-id alternatives; or require a deterministic-prefix pattern; document collision handling                                                                        |
| S7  | `evidence[]` cross-rule                                               | raw-prompt-ref forbids public visibility | also forbid `storage: "transparency-log"` for `type ∈ {raw-prompt-ref, raw-tool-output-ref, trace}`                                                                                        |
| S8  | `extensions` (`schema:84-89`, `cue:161-163`)                          | `additionalProperties: true`             | constrain values to `{uri, digest, mediaType, visibility, ?retention}`; forbid inline blobs                                                                                                |
| S9  | `witness.type` (`schema:469-477`)                                     | includes unverifiable values             | drop `independent-service` and `human-review-system` for v0; or add explicit "not machine-verifiable in v0" semantic                                                                       |
| S10 | `timestamps` ordering                                                 | none                                     | CUE: `finishedAt >= startedAt` (string compare on RFC 3339 works)                                                                                                                          |
| S11 | `pullRequest.url`                                                     | `format: uri`                            | restrict to `^https://github\.com/.*/pull/[0-9]+$` when paired with `executionType: github-actions`; otherwise accept generic URL — but this is a hint, not a hard requirement             |
| S12 | `runtime` (`schema:487-516`)                                          | optional, no link to evidence            | CUE conditional: if `runtime` present, `evidence` must contain a `type: "trace"` entry                                                                                                     |
| S13 | `environment.builder.id` for `executionType: "github-actions"`        | unconstrained 512 chars                  | regex `^https://github\.com/[^/]+/[^/]+/\.github/workflows/[^@]+@.+$` and require `workflowRef`                                                                                            |
| S14 | `materials[].uri` (`schema:867-871`)                                  | unconstrained 1..1024                    | scheme allowlist; document that material URIs are signed/public                                                                                                                            |
| S15 | `evidence[].digest` and `materials[].digest`                          | only `sha256`                            | future-proof with `sha512` optional but pinned, no `md5/sha1` ever; document algorithm-agility plan now to avoid v0→v1 churn                                                               |
| S16 | `privacy.publicTransparencyLog.includesRawContent` (`schema:611-613`) | `const: false` flag is self-asserted     | document this is a *promise* not an *enforcement*; the verifier must additionally compute that no `evidence[]` with raw-* type is present and no `extensions` value carries inline content |

## 4. Policy-level fixes (`policies/default.rego`)

| #   | Issue                                                                    | Fix                                                                                                                                                                                                                                             |
| --- | ------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P1  | No min-level enforcement (B2)                                            | `deny if level_rank[level] < level_rank[context.requiredLevel]`                                                                                                                                                                                 |
| P2  | Subject superset only (B3)                                               | extra deny: every `statement.subject[i]` has corresponding entry in `context.subjects`                                                                                                                                                          |
| P3  | No verified-identity checks (B6)                                         | deny rules consuming `context.verifiedSigner`, `context.verifiedBuilderId`, `context.verifiedWorkflowRef`, `context.verifiedIssuer` for policy-grade and high-assurance                                                                         |
| P4  | No freshness check                                                       | deny if `context.freshnessWindowSeconds > 0` and `context.now - parse_rfc3339(predicate.timestamps.finishedAt) > freshnessWindowSeconds` → emit `stale_attestation`                                                                             |
| P5  | No replay-on-different-PR check                                          | deny if `context.expectedPullRequestNumber != predicate.repo.pullRequest.number` (when both set) → `replay_detected`                                                                                                                            |
| P6  | No envelope→predicate builder cross-check                                | deny if `predicate.environment.builder.id != context.verifiedBuilderId` at policy-grade/high-assurance                                                                                                                                          |
| P7  | `executionType: github-actions` + `runnerClass: self-hosted` not flagged | deny at policy-grade unless `context.allowSelfHostedRunners == true`; otherwise emit `builder_identity_mismatch`                                                                                                                                |
| P8  | Visibility-vs-storage cross-rule missing                                 | deny if `evidence[].storage == "transparency-log"` and `visibility != "public"` (storage/visibility disagreement); deny if `evidence[].type ∈ {trace, raw-prompt-ref, raw-tool-output-ref}` and `storage == "transparency-log"`                 |
| P9  | `extensions` not walked                                                  | deny if any `extensions[k]` lacks the constrained shape (post-S8) or has `visibility == "public"`                                                                                                                                               |
| P10 | `timestamps.finishedAt < startedAt` not flagged                          | deny with `schema_invalid` (or a new `timestamps_invalid`)                                                                                                                                                                                      |
| P11 | `pullRequest.number` self-claim                                          | deny if `context.expectedPullRequestNumber > 0` and `predicate.repo.pullRequest.number != context.expectedPullRequestNumber`                                                                                                                    |
| P12 | No deduplication of `context.subjects`                                   | deny if duplicate names in `context.subjects` (defensive against test-side mistakes)                                                                                                                                                            |
| P13 | Stable failure-code mapping not in policy                                | each `deny msg` should pair a stable code: emit `{code, message}` objects. The current `msg := "..."` format is human-only and contradicts the "failure codes are stable strings" promise (`docs/VERIFICATION_MODEL.md:84-105`, `TASKS.md:108`) |

## 5. Test fixture additions

The current `tests/golden/README.md` lists 6 fixtures. Add at least these 12 — every one targets a hole identified above:

1. `valid-high-assurance/` — currently the high-assurance branch has *no* positive fixture. CUE/JSON Schema conditionals are untested at the green-path.
2. `invalid-extensions-leak/` — raw prompt text smuggled under `extensions["https://vendor.example/v1"].rawPrompt`. Expected: `privacy_violation`.
3. `invalid-data-uri-evidence/` — `evidence[0].uri = "data:text/plain;base64,..."`. Expected: `schema_invalid` post-S1.
4. `invalid-credentials-in-repo-url/` — `repo.url = "https://user:token@github.com/..."`. Expected: `schema_invalid` post-S2.
5. `invalid-level-downgrade/` — predicate `evidence-grade`, context `requiredLevel: policy-grade`. Expected: `level_below_required` (post-B2).
6. `invalid-extra-statement-subject/` — statement has subjects A and B, context expects only A. Expected: `subject_digest_mismatch` (or new `subject_set_mismatch`) post-B3.
7. `invalid-self-asserted-isolated-runner/` — `executionType: "isolated-runner"`, builder.id not in `context.allowedIsolatedRunners`. Expected: `builder_identity_mismatch`.
8. `invalid-witness-without-bundle/` — `witnesses[0].type: "transparency-log"` but no Sigstore bundle digest matches. Expected: `transparency_verification_failed` (post-B5).
9. `invalid-trace-id-without-evidence/` — `runtime.traceIds` set, no `evidence[type=trace]`. Expected: `schema_invalid` post-S12.
10. `invalid-stale-attestation/` — `finishedAt` 7 days old, `freshnessWindowSeconds: 86400`. Expected: `stale_attestation` post-P4.
11. `invalid-builder-mismatch/` — `predicate.environment.builder.id != context.verifiedBuilderId`. Expected: `builder_identity_mismatch` post-B6.
12. `invalid-trace-on-transparency-log/` — `evidence[type=trace, storage=transparency-log]`. Expected: `privacy_violation` post-P8.
13. `invalid-additional-properties-conditional/` — root has unknown key while `verificationLevel: policy-grade`. Verifies B-side of H8.
14. `invalid-failure-ordering/` — both `unsupported_predicate_version` and `subject_digest_mismatch` true. Verifier output's *first* code must be the version one. Verifies H5.

Also: the existing `valid-minimal` says "`local-only` evidence" + "evidence-grade" — that's fine, but make sure the fixture explicitly *fails* when verifier is run with `context.requiredLevel: policy-grade`. Pair the same fixture with two contexts.

## 6. Wording changes (avoid false claims)

| File                                                                                                                                           | Current                                                    | Replace with                                                                                                                                                                                                                                                                                                                                        |
| ---------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/VERIFICATION_MODEL.md:13` ("It proves: A statement claims that one agent run is associated with...")                                     | conflates "claim" and "proof"                              | "It proves that **someone signed** a claim binding one agent run to specific subjects, repo metadata, timestamps, privacy settings, and evidence references — and the signature was not modified after signing."                                                                                                                                    |
| `docs/VERIFICATION_MODEL.md:14` ("If signed, the statement was not modified after signing.")                                                   | "If signed" is ambiguous at evidence-grade                 | call out that evidence-grade *may* be unsigned and that an unsigned evidence-grade attestation proves only "this file existed at this digest at the time it was hashed"                                                                                                                                                                             |
| `docs/THREAT_MODEL.md` row 2 (replay mitigation)                                                                                               | promises run ID, signer identity, timestamps               | reduce promise to what the default policy actually checks (repo URL, base commit, subject digests) and explicitly note "repository policy must add freshness, signer, and PR-context checks" — or implement them in default.rego (P4-P6)                                                                                                            |
| `docs/THREAT_MODEL.md` row 1 ("optionally hash-chain local events and place the root in signed evidence")                                      | aspirational                                               | either remove "hash-chain" claim or move to a separate "future work" section; v0 doesn't define a chain format                                                                                                                                                                                                                                      |
| `docs/PRIVACY_MODEL.md:51-53` ("Public transparency logs are appropriate for ... Minimal non-sensitive statement payloads when policy allows") | undefined "minimal non-sensitive"                          | define it: "the predicate is safe to log iff every `evidence[].visibility != public` for raw-* types AND `extensions` is absent or has all values with non-public visibility AND `agent.declaredIdentity` and `humanApproval.reviewerRefs[]` match the constrained patterns". Make the verifier compute this and emit `privacy_violation` if false. |
| `ARCHITECTURE.md:15` ("indexes... verification results")                                                                                       | invites cached pass/fail                                   | "indexes deterministic refs and digests; never caches pass/fail decisions"                                                                                                                                                                                                                                                                          |
| `AGENTS.md:38` ("Treat agent identity in the predicate as self-asserted unless backed by signer, builder, or workflow identity.")              | good rule, but not enforced                                | add: "The default verifier MUST emit `signer_identity_mismatch` whenever `predicate.agent.declaredIdentity` is non-empty and does not match `context.verifiedSigner`." (or drop `declaredIdentity` from v0)                                                                                                                                         |
| `docs/DATA_MODEL.md:74` ("Policy-grade and high-assurance claims require non-local execution and builder identity.")                           | "builder identity" alone                                   | add "**verified** builder identity from the envelope, not the self-asserted `environment.builder` field"                                                                                                                                                                                                                                            |
| `docs/VERIFICATION_MODEL.md:50-66` (high-assurance)                                                                                            | promises "isolated runner", "witness", "replay resistance" | each needs the corresponding default-policy check; otherwise downgrade prose to "policy profile"-only language. Currently the prose is stronger than the code.                                                                                                                                                                                      |
| `tests/golden/README.md:88-92` (`invalid-replay`)                                                                                              | "the most specific implemented failure code" is fuzzy      | hard-pin one expected code per fixture; "specific implemented" lets implementations diverge silently                                                                                                                                                                                                                                                |
| `TASKS.md:8-16` (Task 1)                                                                                                                       | "schema checks" in CI                                      | name them: `cue vet`, JSON Schema 2020-12 validator, `opa eval`, plus a Go test that runs all golden fixtures end-to-end                                                                                                                                                                                                                            |

## 7. Final go/no-go verdict

**No-go for `v0-alpha` Task 1 as currently specified. Conditional go after the four blockers below close.**

The contract pack is structurally sound — you've made the right meta-decisions (in-toto v1, DSSE/Sigstore reuse, OTel/OpenInference references, SPDX/CycloneDX/OpenVEX sidecars, no custom crypto). The layering, non-goals, and architectural guardrails in `ARCHITECTURE.md` are unusually disciplined for v0. The schema/CUE/Rego/fixture lockstep rule is the right rule. The privacy defaults (`stored: false`, public-rejection for raw evidence) are the right defaults.

But four issues are load-bearing blockers that would let an attacker silently weaken every guarantee the project advertises. They are cheap to fix in this phase and very expensive to fix after Tasks 7–12 ship:

1. **B1** (`extensions` privacy/trust bypass) — closes the only unbounded data channel in the predicate.
2. **B2** (no min-level enforcement / self-downgrade) — without this, every other level-conditional check is moot.
3. **B3** (subject-set superset, not equal) — without this, the central binding claim is under-checked.
4. **B6** (verifier identity not implemented in default policy) — without this, "policy-grade" is purely self-asserted and the entire `THREAT_MODEL.md` row "Forged builder identity" is unmitigated.

Suggested sequence:
- Patch B1, B2, B3, B6 in `schemas/`, `cue`, `default.rego`, and `tests/golden/README.md` first. Add fixtures #1, #2, #5, #6, #11 from §5. Update `docs/VERIFICATION_MODEL.md` and `docs/THREAT_MODEL.md` per §6.
- Then start Task 1 (Go module skeleton). The contract is now safe to lock in.
- Schedule §3, §4, §5 (rest), and §6 fixes inside Tasks 2, 8, 9, 10 — they are aligned to the existing acceptance criteria, not new work.

If the four blockers above are closed in-place (no scope expansion needed, no new dependencies, no replacement of existing wheels), this design is implementable. If they are deferred, every layer downstream of the predicate is going to inherit the holes and re-litigating them at v0-beta or v0 will require a new predicate URI and a fresh fixture set — which is exactly the breakage `AGENTS.md:49` is trying to avoid.