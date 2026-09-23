# Tasks

Implementation is organized by contract generation and capability, not by calendar. Every task
has deterministic acceptance criteria. **No task may rely on AI judgment to decide whether it is
secure.**

This roadmap was overhauled in September 2026 after a frontier-harness review
([`docs/FRONTIER_HARNESS_2026.md`](docs/FRONTIER_HARNESS_2026.md)). The April-2026 roadmap is
superseded: `AGENTS.md`, MCP, harness-native telemetry, and first-party platform coding agents are
now load-bearing, and the predicate model has been extended to **v1** to bind them.

## Status Summary

| Area | State |
|---|---|
| v0 predicate contract (JSON Schema + CUE + Rego) | **shipped** — 34 v0 golden fixtures pass |
| v0 verifier pipeline (phase 01–05, failure ordering) | **shipped** — `internal/verify` |
| Deterministic git binding | **shipped** — `internal/gitbind` |
| SQLite cache (refs/digests/timestamps, no pass/fail) | **shipped** — `internal/cache` |
| Privacy gate (structural, schema + Rego) | **shipped** |
| **v1 predicate contract** (agentConfig, MCP, delegation, capture, platform-agent) | **shipped** — 12 v1 golden fixtures pass |
| DSSE signing + identity adapter (`internal/signing`) | **shipped** — DSSE `Sign`/`Verify`, X.509 SAN/Fulcio-issuer identity, trust-root chain verification, Sigstore/`gh attestation` bundle ingestion, and RFC 6962 Rekor inclusion-proof binding |
| GitHub Action + attestation verification + PR summary | **shipped** — `action/`, `verify bundle`, `summary`, `context` |
| CI (fmt/vet/schema/CUE/Rego/golden + 3-OS `-race` matrix) | **shipped** — `.github/workflows/ci.yml` |
| Release pipeline (`version` + goreleaser + release workflow + CHANGELOG) | **shipped** — `v0.1.0` tagged |
| Sigstore TUF trust-root sourcing (`verify bundle` without `--trust-root`) | **shipped** — `internal/signing/trustroot.go` |
| Harness capture SDK (`internal/capture`) | **shipped** — harness-native v1 capture, no raw content |
| Registry / discovery (`internal/registry`) | **shipped** — read-only, opt-in index; never a trust root |

---

## v0 — foundation (mostly shipped)

### 1. Repository skeleton — DONE
Go module, CLI entry, package layout, schema/policy/test dirs, `ARCHITECTURE.md` boundaries.
Acceptance: `go test ./...` runs; no forbidden import directions; CI runs fmt/vet/schema/CUE/Rego/golden.

### 2. Predicate types & validation — DONE
Go types + JSON Schema 2020-12 + CUE for `https://agentattest.dev/predicate/v0`. Unsupported
versions rejected pre-schema. Acceptance: valid/invalid goldens; closed objects; URI safety;
runtime↔trace binding; context schema validation.

### 3. Deterministic git binding — DONE
`internal/gitbind`: normalized repo URL, base/head, patch sha256, changed-file sha256, cross-platform
deterministic. Acceptance: identical digests on Windows/macOS/Linux; no signing/policy imports.

### 4. Local evidence capture contract — DONE
Default predicate has `rawPrompt.stored=false`, `rawToolOutputs.stored=false`; evidence is
digest-addressed. Acceptance: raw capture is opt-in and never `public`.

### 5. SQLite cache — DONE
`internal/cache` indexes refs/digests/timestamps; never stores authoritative pass/fail.
Acceptance: verification runs without trusting cache; corruption never yields a valid result.

### 6. in-toto Statement assembly — DONE
`internal/statement` + `predicate.StatementFor`. Acceptance: exact `_type`/`predicateType`;
subjects derived from deterministic digests; schema validates before output.

### 7. DSSE / Sigstore signing adapter — SHIPPED (envelope + identity layer)
Isolate all signing/verification of DSSE, cosign, Fulcio/Rekor bundles, and GitHub attestation
verification in `internal/signing`. Expose signer, certificate, issuer, workflow, builder, witness,
approval, and timestamp data as **structured verifier context** (the shape already consumed by
`policies/default.rego`).

Acceptance criteria:
- Only `internal/signing` imports DSSE/cosign/Fulcio/Rekor/GitHub-attestation libraries.
- No custom signing primitive is implemented.
- Tampered statement payloads are rejected.
- Verification produces the `context.verifiedSigner` / `verifiedBuilderId` / `verifiedWorkflowRef` /
  `verifiedIssuer` / `verifiedWitnesses` / `verifiedApprovalDigest` fields the default policy reads.
- Platform-agent signers (e.g. GitHub Copilot coding agent bot identity + issuer) are handled by the
  same generic identity extraction as every other signer — the certificate SAN becomes
  `verifiedSigner` and the Fulcio OIDC extension becomes `verifiedIssuer` — and the `platform-agent`
  builder type is a first-class schema/CUE/policy category pinned by `valid-v1-platform-agent`.

Shipped in `internal/signing`: `Sign` wraps a statement payload in a DSSE envelope via a caller-supplied `crypto.Signer` (secure-systems-lab/go-securesystemslib — no custom crypto); `Verify` verifies the envelope against trusted signer certificates, rejects tampering / missing signatures / wrong keys, and extracts `verifiedSigner` / `verifiedBuilderId` / `verifiedWorkflowRef` / `verifiedIssuer` from the certificate SAN and the Fulcio OIDC extension (`1.3.6.1.4.1.57264.1.1`). `VerifyWithTrustRoot` additionally verifies the leaf certificate chains to a configurable trust root (the Fulcio root CAs) with an optional OIDC-issuer allowlist, and `ParseCertificates` ingests the PEM certificate chain cosign / `gh attestation` expose. Offline tests cover the round trip, identity extraction, trust-root chain verification (accept / untrusted-root / disallowed-issuer / expired-leaf), and fail-closed cases, plus an end-to-end signed policy-grade v1 statement that verifies.

Remaining (next layer): none. Rekor inclusion-proof verification shipped (`VerifyInclusion`, bound via `FromSigstoreBundle` with `--require-inclusion` / `--rekor-root`), and the trust root is either supplied explicitly (`--trust-root`) or sourced from the Sigstore community TUF repository (`LoadTrustRoot`).

### 8. Default Rego policy — DONE
`policies/default.rego`: repo/base/subject-set equality, required level, verified identity, level
escalation, public-log privacy, replay, freshness, high-assurance runner/witness/approval, and the
v1 agentConfig/MCP/delegation gates. Acceptance: each rule has a golden fixture with one code.

### 9. Golden test harness — DONE
`internal/verify/golden_test.go` runs every `tests/golden/*` fixture; invalid fixtures pin exactly
one code; multi-code ordering lives in Go testdata. Acceptance: 46 fixtures stable across runs (34 v0 + 12 v1).

### 10. Privacy gate — DONE
Structural privacy enforced jointly by JSON Schema, CUE, and Rego. Acceptance: public-log blocked
for raw/trace/credentialed/`data:`/inline-blob/public-extension cases.

---

## v1 — frontier-harness contract (shipped in this upgrade)

v1 keeps every v0 invariant and adds binding for the harness plane that consolidated in 2026. See
[`docs/FRONTIER_HARNESS_2026.md`](docs/FRONTIER_HARNESS_2026.md) and
[`docs/DATA_MODEL.md`](docs/DATA_MODEL.md#v1-additions).

### 11. v1 predicate contract — DONE
`https://agentattest.dev/predicate/v1` with `agentConfig`, `mcpServers`, `tools`, `delegation`,
`capture`, and `platform-agent` builder/execution types. Acceptance: JSON Schema + CUE in lockstep;
`additionalProperties:false`; privacy invariants preserved; 8 golden fixtures pass.

### 12. v0/v1 verifier routing — DONE
`internal/verify` phase 01 accepts both predicate types with type/version consistency and routes to
the matching schema + CUE. Acceptance: v0 outcomes unchanged; v1 fixtures pass; unit tests lock
routing and consistency.

### 13. v1 policy gates — DONE
agentConfig binding required at high-assurance (v1); declared agentConfig/MCP/delegation must match
out-of-band verified context when the verifier supplies it. Acceptance: one-code invalid fixtures
for each gate; no v0 regression.

### 14. v1 capture-path depth — DONE (folded into the capture SDK)
`internal/capture` binds harness identity + OTel trace for `harness-native` capture and fails closed when the trace evidence digest is absent or malformed. A hash-chained event root as evidence remains a future enhancement where a harness exposes a signed event log.

---

## v0.1 — integration & forward work

### 15. GitHub Action — SHIPPED (gate + summary; OIDC signing deferred to tasks 16/18)
Shipped: `action/action.yml` + `docs/examples/agentattest.yml` (see `action/README.md`). Composite action wrapping the CLI (no second verifier). Generate a custom-predicate attestation for
patch/tree/artifact; sign via GitHub OIDC/Sigstore; run the verifier; emit a check summary; fail
closed. Acceptance: matches original task 11 criteria plus a `platform-agent` example workflow.

### 16. GitHub attestation verification path — SHIPPED
`internal/signing.FromSigstoreBundle` consumes a Sigstore bundle or `gh attestation verify --format json` output: it pairs the `dsseEnvelope` with its x509 certificate chain, re-verifies the chain against a trusted root (`VerifyWithTrustRoot`), and returns verified context + the decoded statement. `agentattest verify bundle --bundle PATH [--trust-root ROOT.pem]` runs the full gate to policy-grade; without `--trust-root` the Sigstore community root is fetched via TUF. Acceptance met: verified certificate/workflow identity (not user-controlled predicate fields) drives the policy — a self-asserted `declaredIdentity` that disagrees fails with `signer_identity_mismatch` (tested). Rekor inclusion proofs are verified when present and required via `--require-inclusion`.

### 17. PR summary output — DONE
`agentattest summary` (internal/app `renderSummary`) emits a deterministic Markdown summary: result, level, subject digest prefixes, repo/base match, verified signer/builder/issuer (from context), privacy presence flags, and v1 `capture`. Never includes raw prompts/tool outputs/traces/secrets. Acceptance: deterministic (tested), privacy-safe, wired into the GitHub Action.

### 18. Harness capture SDK / exporter — SHIPPED
`internal/capture.Statement(gitbind.Result, RunMetadata)` emits a harness-native v1 statement: agent/model identity, `agentConfig` (AGENTS.md by digest), `runtime` OTel trace refs, a bound `trace` evidence entry, and optional MCP server/tool identity. No raw prompt/tool/trace is captured. Acceptance met: reference integrations for a CLI harness (Claude Code) and a platform agent (Copilot coding agent) in `docs/examples/capture.md` and tests; fails closed without a bound trace.

### 19. Registry / discovery — SHIPPED (still non-goal for v0/v1 cores)
`internal/registry` is a read-only, opt-in SQLite index mapping subject digest / repo URL / runId to statement refs. It stores references and non-raw metadata, never pass/fail decisions, and is never a trust root.

### 20. Release & compatibility contract — SHIPPED (v0.1.0)
Every schema change updates `docs/DATA_MODEL.md`, the CUE file, and ≥1 golden fixture. `AGENTS.md`,
`ARCHITECTURE.md`, `docs/*`, `schemas/*`, `policies/*`, and `tests/golden/*` stay consistent. A
compatibility suite (all goldens) is part of CI (`.github/workflows/ci.yml`). Release automation
ships: `agentattest version` with ldflags injection, `.goreleaser.yaml`, tag-triggered
`.github/workflows/release.yml`, and `CHANGELOG.md`. Tag `v0.1.0` is cut.

### 21. Security review checklist — SHIPPED (evidence in `docs/SECURITY_REVIEW.md`)
No custom crypto; signing only in `internal/signing` (machine-enforced by
`internal/signing/boundary_test.go`); no default raw storage; no public-log raw content;
policy-grade/high-assurance reject local-only evidence; v1 gates fail closed; failure modes
documented and tested. The review closed the gaps found during the v0.1.0 audit: the hand-rolled
RFC 6962 hasher was replaced by `rfc6962.DefaultHasher`, `signature_invalid` is now a structured
result, the `cache_untrusted` ghost code was removed, and the v1 mcpServers/delegation/capture
gates fail closed when declared without verified context.
