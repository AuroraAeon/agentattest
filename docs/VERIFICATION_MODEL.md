# Verification Model

Verification is a set of deterministic checks over a verified envelope, an in-toto Statement, the `agentattest` predicate, current repository context, and repository policy.

`agentattest` verifies provenance claims. It does not prove code quality, model correctness, or total machine integrity.

## Evidence-Grade

Evidence-grade is the lowest assurance level. It is intended for local CLI capture, local development workflows, and early PR evidence.

It proves:

- Someone produced a statement claiming that one agent run is associated with specific subjects, repository metadata, timestamps, privacy settings, and evidence references.
- If signed, the statement payload was not modified after signing.
- If unsigned, it proves only the local file contents and digests presented to the verifier.
- The verifier can compare the statement subjects to current patch, tree, PR, or artifact digests.
- The predicate conforms to the v0 schema and default privacy requirements.

It does not prove:

- The local machine was uncompromised.
- The agent, model, or tool metadata is truthful.
- The run happened in an isolated or controlled environment.
- The change should be merged.

Evidence-grade may use `local-only` evidence.

## Policy-Grade

Policy-grade is intended for CI and GitHub-first merge gates.

It proves:

- The statement is signed or verified through an accepted DSSE/Sigstore/GitHub attestation path when repository policy requires signing.
- The verifier supplied signer, builder, workflow, issuer, repository, or certificate identity as explicit context and the default policy matched it to the predicate where applicable.
- The statement subjects exactly match current patch, tree, PR, or artifact digests.
- The repository URL and base commit match the current verification context.
- The predicate is schema-valid and privacy-safe under structural v0 checks.
- No evidence required for the claim is `local-only`.

It does not prove:

- The CI provider or runner was impossible to compromise.
- A malicious but authorized workflow could not lie in user-controlled predicate fields.
- The generated code is secure or correct.
- Human review was meaningful beyond verified platform review context supplied to policy.

Policy-grade must treat certificate, signer, workflow, and verified timestamp data as stronger than predicate self-assertions.

## High-Assurance

High-assurance is a stricter policy profile for environments with stronger runner controls, witnessed execution, protected merge gates, and stricter identity controls.

It proves:

- All policy-grade checks pass.
- The execution environment is declared as isolated or witnessed and the builder ID appears in `context.allowedIsolatedRunners`.
- At least one witness reference is present and matched to `context.verifiedWitnesses`.
- Human approval is declared as `approved-before-merge`, an approval evidence digest is present, `context.verifiedApproval=true`, and `context.verifiedApprovalDigest` matches that approval evidence digest.
- Optional replay checks for PR number, run ID, and freshness pass when the verifier provides those context fields.

It does not prove:

- Hardware, firmware, hypervisor, or SaaS control planes were impossible to compromise.
- The model's hidden reasoning was faithful or inspectable.
- The code is free of vulnerabilities.
- All possible side-channel or insider threats are eliminated.

High-assurance is a policy profile, not a universal proof of trust.

## Formal Verification Pipeline

Verifier implementations must run these phases in order. Each invariant has one owning phase; later phases must not reclassify an earlier-phase failure.

| Phase | Owner | Checks | Failure codes |
|---|---|---|---|
| 01 | Go verifier pre-schema gate | Decode the statement envelope, verify `_type`, `predicateType`, and `predicate.predicateVersion` routing for this contract pack | `unsupported_statement_type`, `predicate_type_mismatch`, `unsupported_predicate_version` |
| 02 | JSON Schema | Structural predicate and verifier-context shape: required fields, closed objects, enum shapes, safe URI patterns, raw-evidence visibility coupling, GitHub builder string shape, and constrained extension objects | `schema_invalid` |
| 03 | CUE | Cross-field semantic predicate checks not owned by policy: timestamp ordering and runtime-to-trace-evidence binding | `schema_invalid` |
| 04 | Rego policy | Context-bound policy: required level, exact subject set, repo/base match, verified signer/builder/workflow/issuer checks, level escalation, public-log privacy, high-assurance runner/witness/approval checks, replay, and freshness | Rego deny `code` values |
| 05 | Go verifier output | Deduplicate and order failure codes for deterministic user-visible output | no new code |

Phase 04 is the only phase that enforces `level_escalation`. JSON Schema and CUE intentionally allow a policy-grade predicate with local-only evidence or missing high-assurance policy evidence so Rego can emit the stable policy failure code.

## v0 And v1 Routing

The pipeline is identical for both predicate versions. Phase 01 accepts `predicateType` and `predicateVersion` in `{v0, v1}`, requires them to be consistent (a `v1` type with a `v0` version, or vice versa, fails with `unsupported_predicate_version`), and then routes to the matching schema and CUE definition:

| predicateType | Schema | CUE definition |
|---|---|---|
| `https://agentattest.dev/predicate/v0` | `schemas/agent-provenance-v0.schema.json` | `#AgentProvenanceV0` |
| `https://agentattest.dev/predicate/v1` | `schemas/agent-provenance-v1.schema.json` | `#AgentProvenanceV1` |

An unknown `predicateType` fails with `predicate_type_mismatch`. Phases 02–05 and the failure-code vocabulary are shared; v1 adds no new failure codes. The default Rego policy applies v1-specific gates (agentConfig / MCP / delegation) only when the relevant verifier context is present, so v0 outcomes are byte-for-byte unchanged.

## Verification Inputs

The verifier should evaluate:

- DSSE/Sigstore/GitHub attestation verification result.
- in-toto Statement `_type`, `subject`, `predicateType`, and `predicate`.
- JSON Schema and CUE validation results for the predicate.
- Current repo URL and base commit.
- Current subject digests for patch, tree, PR state, or build artifact.
- `context.requiredLevel`, which defines the repository minimum accepted level.
- Verified signer, builder ID, workflow ref, issuer, witness, approval digest, timestamp, PR, and run context derived out of band.
- **v1 only:** `verifiedAgentConfig` (recomputed operating-contract digest), `verifiedMcpServers` (allowlist of approved server manifest digests), and `verifiedDelegation` (allowlist of verified delegation root `agentRef`s), each derived out of band from a verified envelope or recomputation.
- Current privacy policy and public-log safety constraints.
- Default or repository-provided Rego/CUE policy.

`tests/golden/context.schema.json` defines the deterministic test context shape. These fields are verifier input, not predicate input.

## Failure Modes

| Failure | Meaning | Default Handling |
|---|---|---|
| `unsupported_statement_type` | `_type` is not in-toto Statement v1 | fail closed |
| `predicate_type_mismatch` | `predicateType` is not `https://agentattest.dev/predicate/v0` | fail closed |
| `unsupported_predicate_version` | `predicateVersion` is not `v0` | fail closed in phase 01 before JSON Schema, CUE, or policy evaluation |
| `schema_invalid` | Predicate or golden context does not satisfy JSON Schema or CUE | fail closed |
| `signature_invalid` | Envelope signature or certificate chain fails verification | fail closed for policy-grade and high-assurance |
| `transparency_verification_failed` | Rekor, timestamp, bundle, or declared witness verification fails when required | fail closed when policy requires transparency or witnesses |
| `subject_digest_mismatch` | Statement subjects and current subjects are not equal sets | fail closed |
| `repo_url_mismatch` | Predicate repo URL differs from current verification context | fail closed |
| `base_commit_mismatch` | Predicate base commit differs from current base commit | fail closed |
| `signer_identity_mismatch` | Verified signer identity is missing or does not satisfy policy | fail closed |
| `builder_identity_mismatch` | Verified builder/workflow/issuer identity is missing or does not satisfy policy | fail closed |
| `replay_detected` | PR number, run ID, or another explicit replay context check indicates reuse in the wrong context | fail closed |
| `privacy_violation` | Raw prompt/tool output, trace, extension, or transparency-log storage violates structural privacy rules | fail closed |
| `level_escalation` | Predicate claims policy-grade or high-assurance with local-only evidence, local execution, missing builder, missing high-assurance witness, or wrong approval state | fail closed in phase 04 |
| `level_below_required` | Predicate `verificationLevel` is below `context.requiredLevel` | fail closed |
| `missing_evidence` | Required trace, sidecar, approval, witness, or digest reference is absent | fail closed when required by level or policy |
| `stale_attestation` | Statement predates the configured freshness window | fail closed when freshness is required |
| `policy_eval_error` | Policy engine cannot evaluate deterministic inputs | fail closed |
| `cache_untrusted` | SQLite cache entry conflicts with verified statement or current context | ignore cache and verify from source |

Default Rego emits structured deny objects with stable `code` values. Rego sets are unordered, so Go-side verifier code must deduplicate and apply the documented failure ordering before producing user-visible output. Multi-code ordering cases are not golden fixtures; `internal/verify/testdata/unsupported-version-subject-mismatch.json` covers `unsupported_predicate_version` before `subject_digest_mismatch`.
