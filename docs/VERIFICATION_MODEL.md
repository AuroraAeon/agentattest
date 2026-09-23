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
- The predicate conforms to the v0 or v1 schema and default privacy requirements.

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
- The predicate is schema-valid and privacy-safe under structural v0/v1 checks.
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

An unknown `predicateType` fails with `predicate_type_mismatch`. Phases 02–05 and the failure-code vocabulary are shared; v1 adds no new failure codes. v0 outcomes are byte-for-byte unchanged.

The default Rego policy applies the v1-specific gates (agentConfig / MCP servers / delegation) as follows:

- **agentConfig digest:** checked whenever the verifier supplies `verifiedAgentConfig`; additionally required at high-assurance for every v1 predicate.
- **MCP servers and delegation:** checked whenever the verifier supplies `verifiedMcpServers` / `verifiedDelegation`; at policy-grade and above, a v1 predicate that *declares* `mcpServers` or a `delegationChain` without the corresponding verified allowlist fails closed with `missing_evidence` instead of passing unverified. Every delegation step (not just the chain root) must be in the verified set.
- **capture:** `capture.method: manual` is rejected at high-assurance, and high-assurance always requires bound trace evidence.

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
| `predicate_type_mismatch` | `predicateType` is neither `https://agentattest.dev/predicate/v0` nor `https://agentattest.dev/predicate/v1` | fail closed |
| `unsupported_predicate_version` | `predicateVersion` is not `v0` or `v1`, or does not match `predicateType` | fail closed in phase 01 before JSON Schema, CUE, or policy evaluation |
| `schema_invalid` | Predicate or golden context does not satisfy JSON Schema or CUE | fail closed |
| `signature_invalid` | Envelope signature or certificate chain fails verification (emitted as a structured result by `verify bundle`) | fail closed for policy-grade and high-assurance |
| `transparency_verification_failed` | A declared witness is not backed by `context.verifiedWitnesses` | fail closed when policy requires witnesses |
| `subject_digest_mismatch` | Statement subjects and `context.subjects` are not equal sets, no subjects were provided, or `context.subjects` contains duplicate names | fail closed |
| `repo_url_mismatch` | Predicate repo URL differs from `context.repoUrl` | fail closed |
| `base_commit_mismatch` | Predicate base commit differs from `context.baseCommit` | fail closed |
| `signer_identity_mismatch` | Verified signer identity is missing or does not satisfy policy | fail closed |
| `builder_identity_mismatch` | Verified builder/workflow/issuer identity is missing or does not satisfy policy | fail closed |
| `replay_detected` | PR number, run ID, or another explicit replay context check indicates reuse in the wrong context | fail closed |
| `privacy_violation` | Evidence or extension violates structural privacy rules — see PRIVACY_MODEL.md for the schema/CUE vs Rego split | fail closed |
| `level_escalation` | Predicate claims policy-grade or high-assurance with local-only evidence, local execution, missing builder, missing high-assurance witness, wrong approval state, or `capture.method: manual` at high-assurance | fail closed in phase 04 |
| `level_below_required` | Predicate `verificationLevel` is below `context.requiredLevel` | fail closed |
| `missing_evidence` | Required trace, sidecar, approval, witness, agentConfig, MCP-server allowlist, delegation-step allowlist, or digest reference is absent or unverifiable | fail closed when required by level or policy |
| `stale_attestation` | Statement predates the configured freshness window, or a window is configured but `context.now` is missing | fail closed when freshness is required |
| `policy_eval_error` | Policy engine cannot evaluate deterministic inputs | fail closed |

There is no `cache_untrusted` code: the verifier never consults the SQLite cache when deciding a result, so a cache conflict cannot produce a failure code — verification always runs from source.

Default Rego emits structured deny objects with stable `code` values. Rego sets are unordered, so Go-side verifier code must deduplicate and apply the documented failure ordering before producing user-visible output. Multi-code ordering cases are not golden fixtures; `internal/verify/testdata/unsupported-version-subject-mismatch.json` covers `unsupported_predicate_version` before `subject_digest_mismatch`.

## Trust Root Sourcing

`agentattest verify bundle` resolves the trusted Fulcio root CA set in one of two ways:

1. **Explicit** — `--trust-root ROOT.pem` supplies PEM certificate(s). This always wins and is the air-gapped path.
2. **Sigstore TUF** — when `--trust-root` is omitted, the root is fetched from the Sigstore community TUF repository (`https://tuf-repo-cdn.sigstore.dev`) via `github.com/sigstore/sigstore-go/pkg/tuf`. The TUF root metadata is pinned by the copy embedded in that library, so the fetch is not trust-on-first-use; metadata and targets are hash-verified per the TUF specification. The metadata cache lives in `os.UserCacheDir()/agentattest/tuf`.

Every failure mode — network unreachable, repository unavailable, malformed or missing `fulcio_v1.crt.pem` / `fulcio.crt.pem` target — fails closed with `signature_invalid` and a non-zero exit. The verifier never falls back to an unverified root.

## Structured Output Of `verify bundle`

`verify bundle` always writes a verifier result to stdout:

- Success and policy failures: the standard `Result` JSON (`valid`, `level`, `failureCodes`).
- Signature/chain/trust-root failures: `{"valid": false, "failureCodes": ["signature_invalid"]}` with the human-readable cause on stderr.

File-not-found and flag errors remain plain `error:` messages on stderr with exit code 1/2 and no result JSON, since no verification was attempted.
