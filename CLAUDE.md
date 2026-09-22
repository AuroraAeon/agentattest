# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Status

This is `agentattest`: a thin interoperability layer for AI coding-agent provenance, with a working Go module (`go 1.23`) plus JSON Schema, CUE, a Rego policy, and golden fixtures. The **v0** contract and the 5-phase Go verifier are shipped and tested (41 golden fixtures pass). The **v1** contract (`https://agentattest.dev/predicate/v1`), which binds the 2026 frontier-harness plane, shipped in the September-2026 overhaul — see [`docs/FRONTIER_HARNESS_2026.md`](docs/FRONTIER_HARNESS_2026.md) and the current [`TASKS.md`](TASKS.md). Still ahead: the `internal/signing` DSSE/Sigstore/GitHub-attestation adapter, the GitHub Action, and PR summary output.

When adding code, the planned stack is: Go for CLI/verifier, JSON Schema + CUE for the predicate, Rego (and optionally CUE) for policy, SQLite for cache, GitHub Action wrapper. Do not introduce other languages or runtimes without revisiting `ARCHITECTURE.md`.

## What To Read Before Any Change

`AGENTS.md` is the entry map and lists strict rules. Always read it. Then, depending on what you are touching:

- Predicate field, schema, or CUE: `docs/DATA_MODEL.md` + `schemas/agent-provenance-v0.schema.json` + `schemas/agent-provenance-v0.cue` (must stay in lockstep) + add/update `tests/golden/`.
- Verifier behavior or failure codes: `docs/VERIFICATION_MODEL.md` (failure codes are stable strings — do not rename).
- Policy: `policies/default.rego` + `docs/VERIFICATION_MODEL.md`.
- Captured data, evidence, raw prompt/tool output handling: `docs/PRIVACY_MODEL.md` + `docs/THREAT_MODEL.md`.
- Scope expansion: `docs/NON_GOALS.md` (hard boundaries — do not negotiate).
- New dependency or package: `ARCHITECTURE.md` (allowed/forbidden import directions).
- What to reuse vs. build: `docs/EXISTING_WHEELS.md`.

## Architecture (Big Picture)

`agentattest` produces an **in-toto Statement v1** whose custom predicate type is exactly `https://agentattest.dev/predicate/v0`. The project owns **only** the predicate, deterministic subject-binding rules, verifier inputs/failure codes, default policy, and CLI/Action ergonomics. Everything else (signing, transparency, traces, SBOM, VEX) is delegated to existing standards.

The eight planned layers (per `ARCHITECTURE.md`) are: **capture → binding → predicate → attestation → verification → policy → cache → integration**. Dependencies flow inward (orchestration → domain) then outward through adapters. Critical forbidden directions:

- Predicate packages must not import Sigstore/cosign/Rekor/SQLite/OPA.
- Only `internal/signing` may import DSSE/cosign/Fulcio/Rekor/GitHub-attestation libraries; no other package implements signing primitives (only hashing/digest is allowed elsewhere).
- Policy packages must not import signing, git, cache, or network adapters.
- Git binding must not import signing, policy, trace, or cache.
- Cache code must not decide verification success.
- The GitHub Action must not contain a second verifier — it wraps the CLI.

## Three Verification Levels

The verifier's behavior is gated by `predicate.verificationLevel`:

- **evidence-grade** — local capture; may use `local-only` evidence; lowest assurance.
- **policy-grade** — non-local execution required; verified builder/signer identity required; `local-only` evidence forbidden.
- **high-assurance** — adds `isolated-runner`/`witnessed-runner` execution, an allowlisted runner, at least one machine-verifiable witness reference, verified witness context, approval evidence, and verified approval context.

`policies/default.rego` enforces these gates in verification Phase 04. JSON Schema owns structural validation, CUE owns cross-field schema semantics, and Rego owns context-bound policy decisions such as level escalation.

## Hard Invariants (Will Trip You Up)

- `predicateType` must be exactly `https://agentattest.dev/predicate/v0` **or** `https://agentattest.dev/predicate/v1`, with a matching `predicate.predicateVersion` (`v0` / `v1`). The Go pre-schema gate rejects unknown types and type↔version mismatches before JSON Schema, CUE, or Rego. Breaking changes require a **new** URI and new fixtures, not in-place edits.
- Every predicate object has `additionalProperties: false`. `extensions` is URI-keyed, but each value is still a closed digest-addressed reference object; arbitrary extension blobs are not allowed.
- A schema change without a matching CUE update **and** a golden fixture update is a broken change.
- Failure codes in `docs/VERIFICATION_MODEL.md` are part of the public contract — verifier output must use those exact strings.
- Raw prompt / raw tool-output visibility must never be `public`. If `stored=false`, visibility must be `none`. JSON Schema and CUE own this structural invariant.
- `local-only` evidence is allowed only at evidence-grade. Rego owns this policy invariant and must emit `level_escalation`.
- Verifier must reject unsupported predicate versions **before** policy evaluation, and reject subject-digest mismatches **before** approval-state checks.
- Treat `agent`/`model` predicate fields as self-asserted metadata, never as a trust root. Trust comes from verified envelope, signer/builder/workflow identity, and recomputed subject digests. If `agent.declaredIdentity` is present, default policy requires it to match `context.verifiedSigner`.
- Verified envelope/certificate data outranks anything in the predicate. Cache contents are never authoritative and must not store pass/fail decisions as trust signals.

## Schema/CUE/Policy/Fixture Lockstep

The predicate is described in three places that must agree:

1. `schemas/agent-provenance-v0.schema.json` — structural validation, `additionalProperties: false`, basic types.
2. `schemas/agent-provenance-v0.cue` — stricter constraints (regexes, level-conditional shape, raw-evidence visibility coupling).
3. `policies/default.rego` — Phase 04 runtime/context checks (repo URL, base commit, exact subject set, required level, verified identity context, level escalation, public-log privacy, freshness, approval digest binding).
4. `schemas/agent-provenance-v1.schema.json` + `schemas/agent-provenance-v1.cue` — the v1 superset (`agentConfig` / `mcpServers` / `tools` / `delegation` / `capture` / `platform-agent`), kept in lockstep with each other; `policies/default.rego` adds the v1 agentConfig / MCP / delegation gates when the matching verifier context is present.

Plus `tests/golden/` fixtures (`valid-minimal`, `valid-github-ci`, `invalid-replay`, `invalid-subject-mismatch`, `invalid-raw-prompt-leakage`, `invalid-level-escalation`) must round-trip through all three. Adding a required field without updating fixtures is expected to fail CI.

## Common Commands

The Go module exists. Canonical commands:

- `go build ./...` — build.
- `go vet ./...` — vet.
- `go test ./...` — full test run (golden fixtures cover schema + CUE + Rego for both v0 and v1).

Schema / CUE / policy checks run through the Go test pipeline (the `santhosh-tekuri/jsonschema`, `cuelang.org/go`, and `open-policy-agent/opa` libraries), so `go test ./...` is the single source of truth. To validate by hand:

- JSON Schema: any draft-2020-12 validator against `schemas/agent-provenance-v0.schema.json`.
- CUE: `cue vet schemas/agent-provenance-v0.cue <fixture>`.
- Rego: `opa eval -d policies/default.rego -i <input.json> 'data.agentattest.default.allow'`.

If you add tooling, prefer wiring it into `go test` / CI rather than introducing parallel runners.

## Don't

- Don't invent crypto, key formats, signing primitives, transparency logs, attestation containers, trace protocols, SBOM/VEX formats, or observability backends. Reuse the tools listed in `docs/EXISTING_WHEELS.md`.
- Don't add raw-prompt or raw-tool-output capture as a default. It is opt-in only and never `public`.
- Don't widen scope without checking `docs/NON_GOALS.md`. VSCode extension, general artifact registry, secret manager, and code-quality claims are explicitly out for v0.
- Don't use shell-interpolated strings for external commands — argv arrays only.
- Don't put `crypto/*` use outside `internal/signing` except for hashing/digest helpers.
