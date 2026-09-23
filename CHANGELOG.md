# Changelog

All notable changes to `agentattest` are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to semantic versioning for the CLI and the predicate contract URIs
(`https://agentattest.dev/predicate/v0`, `https://agentattest.dev/predicate/v1`).

## [0.1.0] - 2026-09-19

First release. Ships the v0 and v1 predicate contracts, the five-phase
deterministic verifier, DSSE/Sigstore signing and bundle verification, the
GitHub Action, the harness capture SDK, and the release pipeline.

### Added

- **Predicate contracts**
  - v0: `https://agentattest.dev/predicate/v0` — JSON Schema 2020-12 + CUE in lockstep, closed objects, URI safety, privacy invariants.
  - v1: `https://agentattest.dev/predicate/v1` — frontier-harness bindings: `agentConfig` (AGENTS.md operating contract by digest), `mcpServers`/`tools` (MCP identity + schema digests), `delegation`, `capture`, and `platform-agent` builder/execution types.
  - v0/v1 verifier routing with type↔version consistency, rejected before schema evaluation.
- **Verifier** (`internal/verify`) — five-phase pipeline (routing, JSON Schema, CUE, Rego policy, failure-code ordering) with stable, ordered failure codes.
- **Default policy** (`policies/default.rego`) — required-level floor, exact subject-set equality, repo/base binding, verified signer/builder/workflow/issuer gates, level escalation, public-log privacy, witness and approval checks, replay and freshness.
- **Signing** (`internal/signing`) — DSSE sign/verify via go-securesystemslib, X.509 SAN/Fulcio-OIDC identity extraction, trust-root chain verification, Sigstore/`gh attestation` bundle ingestion, RFC 6962 Rekor inclusion-proof binding.
- **GitHub Action** (`action/`) — composite action wrapping the CLI; PR check summary; platform-agent example workflow.
- **Harness capture SDK** (`internal/capture`) — harness-native v1 capture binding OTel trace references; fails closed without a bound trace; never stores raw content.
- **Registry** (`internal/registry`) — opt-in, read-only SQLite index of subject digests to statement refs; never a trust root, never stores pass/fail.
- **CLI** — `init`, `digest`, `context`, `predicate create`, `verify predicate`, `verify bundle`, `summary`, `version`.
- **CI** (`.github/workflows/ci.yml`) — fmt/vet/build, contract gates (schema/CUE/Rego/golden via `go test`), and a `-race` matrix on Ubuntu/macOS/Windows.
- **Release pipeline** — GoReleaser config, tag-triggered release workflow, `agentattest version` with build-time ldflags injection.

### Security properties

- No custom cryptography; signing only in `internal/signing`.
- Raw prompts and tool outputs are not stored by default; `public` visibility for raw evidence is structurally impossible.
- Policy-grade and high-assurance reject local-only evidence and self-asserted identities.
- See `docs/SECURITY_REVIEW.md` for the checklist evidence.

[0.1.0]: https://github.com/AuroraAeon/agentattest/releases/tag/v0.1.0
