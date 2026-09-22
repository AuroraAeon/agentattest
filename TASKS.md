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
| v0 predicate contract (JSON Schema + CUE + Rego) | **shipped** — 33 golden fixtures pass |
| v0 verifier pipeline (phase 01–05, failure ordering) | **shipped** — `internal/verify` |
| Deterministic git binding | **shipped** — `internal/gitbind` |
| SQLite cache (refs/digests/timestamps, no pass/fail) | **shipped** — `internal/cache` |
| Privacy gate (structural, schema + Rego) | **shipped** |
| **v1 predicate contract** (agentConfig, MCP, delegation, capture, platform-agent) | **shipped in this upgrade** — 8 new golden fixtures pass |
| DSSE signing + identity adapter (`internal/signing`) | **shipped** — `Sign`/`Verify` over DSSE with X.509 SAN/Fulcio-issuer identity extraction; full Fulcio/Rekor root + bundle verification pending |
| GitHub Action + attestation verification + PR summary | **not started** |
| Harness-native capture exporter / SDK | **planned** |

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
- Platform-agent signers (e.g. GitHub Copilot coding agent bot identity + issuer) are recognized as
  a first-class verified-signer category for the `platform-agent` builder type.

Shipped in `internal/signing`: `Sign` wraps a statement payload in a DSSE envelope via a caller-supplied `crypto.Signer` (secure-systems-lab/go-securesystemslib — no custom crypto); `Verify` verifies the envelope against trusted signer certificates, rejects tampering / missing signatures / wrong keys, and extracts `verifiedSigner` / `verifiedBuilderId` / `verifiedWorkflowRef` / `verifiedIssuer` from the certificate SAN and the Fulcio OIDC extension (`1.3.6.1.4.1.57264.1.1`). `VerifyWithTrustRoot` additionally verifies the leaf certificate chains to a configurable trust root (the Fulcio root CAs) with an optional OIDC-issuer allowlist, and `ParseCertificates` ingests the PEM certificate chain cosign / `gh attestation` expose. Offline tests cover the round trip, identity extraction, trust-root chain verification (accept / untrusted-root / disallowed-issuer / expired-leaf), and fail-closed cases, plus an end-to-end signed policy-grade v1 statement that verifies.

Remaining (next layer): Rekor inclusion-proof verification, and sourcing the trust root (via Sigstore TUF) and the leaf certificate from real Sigstore/cosign bundles and `gh attestation` output.

### 8. Default Rego policy — DONE
`policies/default.rego`: repo/base/subject-set equality, required level, verified identity, level
escalation, public-log privacy, replay, freshness, high-assurance runner/witness/approval, and the
v1 agentConfig/MCP/delegation gates. Acceptance: each rule has a golden fixture with one code.

### 9. Golden test harness — DONE
`internal/verify/golden_test.go` runs every `tests/golden/*` fixture; invalid fixtures pin exactly
one code; multi-code ordering lives in Go testdata. Acceptance: 41 fixtures stable across runs.

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

### 14. v1 capture-path depth — PLANNED
When `capture.method == "harness-native"`, bind the harness identity + OTel trace and (where the
harness exposes a signed event log) a hash-chained event root as evidence. Acceptance: a
harness-native fixture that fails closed if the trace evidence digest is absent or mismatched.

---

## v0.1 — integration & forward work

### 15. GitHub Action — PLANNED
Composite action wrapping the CLI (no second verifier). Generate a custom-predicate attestation for
patch/tree/artifact; sign via GitHub OIDC/Sigstore; run the verifier; emit a check summary; fail
closed. Acceptance: matches original task 11 criteria plus a `platform-agent` example workflow.

### 16. GitHub attestation verification path — PLANNED
Consume `gh attestation verify` / attestation API output as verifier context. Acceptance: verified
certificate/workflow data distinguished from user-controlled predicate fields.

### 17. PR summary output — PLANNED
Deterministic PR/check summary: result, level, subject digest prefixes, repo/base status,
signer/workflow status, privacy status, and v1 `agentConfig`/`capture` presence. Never includes raw
prompts/tool outputs/traces/secrets. Acceptance: golden-deterministic output.

### 18. Harness capture SDK / exporter — PLANNED
A thin library harnesses embed to emit run metadata + OTel trace references that populate a v1
predicate (harness-native capture). Acceptance: reference integration for one CLI harness and one
platform agent; no raw content captured by default.

### 19. Registry / discovery — PLANNED (non-goal for v0/v1 cores)
Optional index by subject digest / repo / runId for retrieval. Acceptance: read-only; never a trust
root; opt-in.

### 20. Release & compatibility contract — ONGOING
Every schema change updates `docs/DATA_MODEL.md`, the CUE file, and ≥1 golden fixture. `AGENTS.md`,
`ARCHITECTURE.md`, `docs/*`, `schemas/*`, `policies/*`, and `tests/golden/*` stay consistent. A
compatibility suite (all goldens) is part of CI.

### 21. Security review checklist — ONGOING
No custom crypto; signing only in `internal/signing`; no default raw storage; no public-log raw
content; policy-grade/high-assurance reject local-only evidence; v1 gates fail closed; failure modes
documented and tested.
