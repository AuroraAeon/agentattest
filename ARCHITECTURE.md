# Architecture

`agentattest` is a binding and verification layer. It connects an AI coding-agent run to a git patch, tree, pull request, build artifact, and human approval state by producing an in-toto Statement v1 with the custom predicate `https://agentattest.dev/predicate/v0`.

It must stay thin. Cryptographic signing, transparency logs, artifact attestation distribution, runtime tracing, SBOMs, VEX, and policy engines are reused from existing ecosystems.

## Layered Architecture

1. Capture layer records local or CI facts about a run: run ID, repo URL, base commit, changed subjects, trace references, sidecar references, and approval state.
2. Binding layer computes deterministic git, patch, tree, and artifact digests and binds those values to the in-toto Statement `subject` array.
3. Predicate layer creates and validates only the agentattest custom predicate.
4. Attestation layer wraps the predicate in in-toto Statement v1 and delegates signing/envelope handling to DSSE, Sigstore/cosign, or GitHub Artifact Attestations.
5. Verification layer verifies the envelope, statement, predicate schema, subject digests, repo context, base commit, signer or builder identity, privacy rules, and policy.
6. Policy layer evaluates deterministic Rego or CUE rules against verified statement data and local verification context.
7. Cache layer indexes local run metadata, evidence references, content-addressed statement refs, and last-seen timestamps in SQLite; it never caches authoritative pass/fail decisions.
8. Integration layer exposes the CLI, GitHub Action, CI wiring, and machine-readable outputs.

## Module Boundaries

Future implementation should keep these boundaries stable.

| Module | Owns | Must Not Own |
|---|---|---|
| `cmd/agentattest` | CLI flags, command routing, exit codes | schema semantics, signing logic, policy decisions |
| `internal/app` | use-case orchestration | direct crypto, direct database SQL, direct shell parsing |
| `internal/gitbind` | repo URL normalization, base commit checks, patch/tree/artifact digest calculation | signing, policy, trace parsing |
| `internal/predicate` | Go representation and validation of `https://agentattest.dev/predicate/v0` | in-toto envelope verification, policy decisions |
| `internal/statement` | in-toto Statement v1 assembly and parsing | custom crypto, privacy redaction |
| `internal/signing` | adapters for DSSE, Sigstore/cosign, Fulcio/Rekor bundles, GitHub attestation verification | predicate mutation, git digest calculation |
| `internal/verify` | verification pipeline and failure mapping | raw prompt capture, signing implementation |
| `internal/policy` | Rego/CUE input shaping and evaluation | network access, git operations |
| `internal/privacy` | field classification, redaction, public-log safety checks | signing, policy engine implementation |
| `internal/trace` | OpenTelemetry/OpenInference reference validation and digest handling | trace UI, trace storage platform |
| `internal/cache` | SQLite schema, indexing, query APIs for refs, digests, and timestamps | verification truth, pass/fail decisions, cryptographic trust |
| `action/` | GitHub Action wrapper and workflow integration | verifier logic not available in the CLI |
| `schemas/` | JSON Schema and CUE contracts | runtime code |
| `policies/` | default policy documents | hard-coded verifier invariants |
| `tests/golden/` | fixtures for schema and policy compatibility | generated production state |

## Allowed Dependency Directions

Dependencies flow inward from interfaces and orchestration toward pure domain logic, then outward through adapters.

Allowed directions:

- `cmd/agentattest` -> `internal/app`.
- `internal/app` -> `internal/gitbind`, `internal/predicate`, `internal/statement`, `internal/signing`, `internal/verify`, `internal/policy`, `internal/privacy`, `internal/trace`, `internal/cache`.
- `internal/verify` -> `internal/statement`, `internal/predicate`, `internal/gitbind`, `internal/signing`, `internal/policy`, `internal/privacy`.
- `internal/statement` -> `internal/predicate`.
- `internal/policy` -> policy engine libraries only.
- `internal/cache` -> SQLite driver only.
- `action/` -> CLI binary or stable machine-readable CLI output.

Forbidden directions:

- Pure model packages must not import CLI packages.
- Policy packages must not import signing, git, cache, or network adapters.
- Predicate packages must not import Sigstore, GitHub, SQLite, or OPA.
- Git binding packages must not import signing, policy, trace, or cache.
- Signing adapters must not compute git digests or mutate predicates.
- Cache code must not decide verification success.
- GitHub Action code must not contain a second verifier implementation.

## Deterministic Constraints For Future Linters

- `predicateType` must be the exact string `https://agentattest.dev/predicate/v0` for v0 statements.
- `predicate.predicateVersion` must be the exact string `v0`; verifier code must reject unsupported versions before JSON Schema, CUE, or policy evaluation.
- The custom predicate schema must keep `additionalProperties: false` except explicitly named `extensions` maps, and extension values must be constrained digest-addressed references rather than arbitrary objects.
- Every schema change must update `docs/DATA_MODEL.md`, `schemas/agent-provenance-v0.cue`, and at least one golden fixture.
- Verifier code must reject unsupported predicate versions before policy evaluation.
- Verifier code must reject subject digest mismatches before evaluating approval state.
- Policy-grade and high-assurance verification must reject `local-only` evidence in Rego with `level_escalation`.
- High-assurance verification must require at least one machine-verifiable witness reference, verified witness context, approval evidence, verified approval context, and a non-local allowlisted execution environment.
- Raw prompt and raw tool-output fields must never have `public` visibility.
- Public transparency-log payloads must never contain raw prompts, raw tool outputs, secrets, or unredacted personal data.
- Only `internal/signing` may import DSSE, cosign, Fulcio, Rekor, or GitHub attestation verification libraries.
- Only hashing and digest utilities may use `crypto/*` outside `internal/signing`; no package may implement signing primitives.
- External command execution must use argv arrays, not shell-interpolated strings.
- Network access in verification must be explicit, opt-in, and isolated in adapter packages.
- SQLite must be used only for cache/index refs, digests, paths, and timestamps; pass/fail decisions must be recomputed and never trusted from cache.
- Trace content is referenced by URI and digest; agentattest must not become a trace storage or observability backend.
- SBOM and VEX documents are sidecars; their schemas must not be embedded into the agent provenance predicate.
