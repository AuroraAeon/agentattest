<div align="center">

# agentattest

**A thin, verifiable provenance layer for AI coding-agent runs.**

**English** &nbsp;|&nbsp; [简体中文](./README.zh-CN.md)

[![status](https://img.shields.io/badge/status-v0%20%2B%20v1-1f6feb)]()
[![predicate](https://img.shields.io/badge/predicate-v0%20%2B%20v1-blue)]()
[![statement](https://img.shields.io/badge/container-in--toto%20Statement%20v1-1f6feb)]()
[![go](https://img.shields.io/badge/go-1.25-00ADD8)]()
[![schema](https://img.shields.io/badge/schema-JSON%20Schema%20%2B%20CUE-8b5cf6)]()
[![policy](https://img.shields.io/badge/policy-OPA%2FRego-7d3aed)]()
[![cache](https://img.shields.io/badge/cache-SQLite-003B57)]()

</div>

---

## TL;DR

`agentattest` binds an AI coding-agent run to a concrete git patch, tree, PR, and approval state by emitting an **in-toto Statement v1** carrying a custom predicate `https://agentattest.dev/predicate/v0`.

**v1** (`https://agentattest.dev/predicate/v1`) additionally binds the frontier-harness plane that consolidated in 2026 — the `AGENTS.md` operating contract, MCP tool/server identity with schema digests, harness-native capture, platform-agent identities (e.g. GitHub Copilot coding agent), and multi-agent delegation. **v0 is unchanged and stable.** See [`docs/FRONTIER_HARNESS_2026.md`](docs/FRONTIER_HARNESS_2026.md).

It owns *only* the predicate, deterministic subject-binding rules, verifier inputs and failure codes, a default Rego policy, and CLI/Action ergonomics.

**Cryptography, transparency logs, traces, SBOM, and VEX are delegated to existing standards** — DSSE, Sigstore/cosign, Fulcio/Rekor, GitHub Artifact Attestations, OpenTelemetry/OpenInference, SPDX, CycloneDX, OpenVEX. No new wheels.

---

## Table of Contents

- [Why](#why)
- [Non-Goals](#non-goals)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [CLI](#cli)
- [Data Model](#data-model)
- [Verification Pipeline](#verification-pipeline)
- [Verification Levels](#verification-levels)
- [Failure Codes](#failure-codes)
- [Privacy Defaults](#privacy-defaults)
- [Project Layout](#project-layout)
- [Existing Wheels](#existing-wheels)
- [Roadmap](#roadmap)
- [Hard Invariants](#hard-invariants)
- [License](#license)
- [Contributing](#contributing)

---

## Why

AI coding agents now author large fractions of real PRs, but the only post-hoc tooling the ecosystem offers is **fingerprinting** — guess-who forensics over diff style. Repository owners need a **first-party, verifiable claim** that ties a run to a diff, plus a **deterministic policy gate** that fails closed on subject, repo, base-commit, identity, privacy, or level violations.

`agentattest` is that gate, and *only* that gate.

| Problem | What `agentattest` does | What it does **not** do |
|---|---|---|
| "Was this PR produced by an agent?" | Emit a signed claim binding run → patch / tree / PR / artifact | Identify agents heuristically from style |
| "Did the same workflow really build this?" | Cross-check verified signer / builder / workflow against predicate | Re-implement Sigstore or cosign |
| "Was raw prompt content leaked?" | Reject `public` raw evidence and credentialed / `data:` URIs by schema + policy | Scan referenced artifact bytes for secrets |
| "Was this attestation replayed?" | Bind repo / base / PR / run / freshness as deterministic verifier context | Run a transparency log of its own |

---

## Non-Goals

`agentattest` will **not** invent or replace any of the following — see [`docs/NON_GOALS.md`](docs/NON_GOALS.md).

- New cryptography, key formats, PKI, timestamping, or transparency logs.
- A new attestation container — it stays on **in-toto Statement v1**.
- A new trace protocol — references **OpenTelemetry / OpenInference** by URI + digest.
- A new SBOM / VEX format — links **SPDX / CycloneDX / OpenVEX** as sidecars.
- An observability platform, code-quality oracle, secret manager, or VSCode extension (in v0).
- A trust root for `agent.name` / `model.name` — those are self-asserted metadata only.

Trust comes from verified signer / builder / workflow identity plus recomputed subject digests, never from anything self-asserted in the predicate.

---

## Architecture

Eight layers; dependencies flow inward (orchestration → domain) then outward through adapters. See [`ARCHITECTURE.md`](ARCHITECTURE.md) for forbidden import directions.

```
┌──────────────────────── integration ─────────────────────────┐
│                  cmd/agentattest · action/                   │
└────────────────────────────┬─────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────┐
│                    internal/app  (orchestration)             │
└──────┬───────────────────────────────────────────────┬───────┘
       │                                               │
   ┌───▼─────┐  ┌────────┐  ┌───────────┐  ┌────────┐  │
   │ gitbind │  │ predi- │  │ statement │  │ verify │  │
   │ (capt.+ │  │ cate   │  │ (in-toto) │  │ (5 ph) │  │
   │ binding)│  │ (v0)   │  └─────┬─────┘  └───┬────┘  │
   └───┬─────┘  └────┬───┘        │            │       │
       │            │             │            │       │
       │            │      ┌──────▼──────┐  ┌──▼─────┐ │
       │            │      │  signing    │  │ policy │ │
       │            │      │ (DSSE/cosign│  │ (Rego) │ │
       │            │      │  /Sigstore) │  └────────┘ │
       │            │      └─────────────┘             │
       │            │                                  │
   ┌───▼────────────▼──────────────────────────────────▼─────┐
   │                   internal/cache (SQLite)               │
   │   refs · digests · timestamps  ─ never trusts pass/fail │
   └─────────────────────────────────────────────────────────┘
```

**Hard rules**

- Predicate packages do not import Sigstore / Rekor / SQLite / OPA.
- Only `internal/signing` may import DSSE / cosign / Fulcio / Rekor.
- Cache code never decides verification success.
- The GitHub Action wraps the CLI, never re-implements the verifier.

---

## Quick Start

### Prerequisites

- Go ≥ 1.25
- `git` on `PATH`
- (optional) `cue` and `opa` for hand validation outside Go tests

### Build & test

```bash
go build ./cmd/agentattest
go test ./...
```

### End-to-end against this repo

```bash
# 1. Initialize the local cache + .agentattest config
#    (no secrets, no pass/fail decisions stored)
./agentattest init --dir .

# 2. Compute deterministic git binding
#    repo URL · base · head · patch digest · changed files
./agentattest digest --repo .

# 3. Emit an in-toto Statement v1 with
#    predicateType = https://agentattest.dev/predicate/v0
./agentattest predicate create --repo . --out statement.json

# 4. Verify against verifier-context JSON
#    (subjects, repoUrl, baseCommit, requiredLevel, ...)
./agentattest verify predicate \
    --statement statement.json \
    --context  tests/golden/valid-minimal/context.json
```

The verifier exits **non-zero** on any failure code and emits a stable JSON result of shape `{ valid, level, failureCodes[] }`.

---

## CLI

| Command | Purpose |
|---|---|
| `agentattest init [--dir DIR]` | Create `.agentattest/` with a SQLite cache and a JSON config. The cache holds **only** refs / digests / timestamps — never pass/fail decisions. |
| `agentattest digest [--repo DIR]` | Compute repo URL, branch, base/head commits, patch SHA-256, changed-file SHA-256s. Cross-platform deterministic. |
| `agentattest predicate create [--repo DIR] [--out PATH] [--version v0\|v1] [--repo-url ...] [--base-commit ...] [--agent-name ...] [--agent-version ...]` | Build a predicate + in-toto Statement v1 with `subject[] = { patch.diff: sha256 }`. Defaults: evidence-grade, local execution, no raw evidence. `--version v1` adds `--capture-method wrapper\|ci-step\|manual`, `--capture-harness NAME`, `--agent-config PATH` (binds `AGENTS.md` by digest), and `--model-provider P --model-id M`. |
| `agentattest verify predicate --statement PATH --context PATH` | Run the 5-phase pipeline. Returns `{ valid, level, failureCodes[] }` JSON. |

The CLI delegates to `internal/app`; `cmd/agentattest/main.go` is a six-line entry point.

---

## Data Model

The signed claim is an **in-toto Statement v1**. The custom JSON Schema applies *only* to the inner `predicate` object — the outer container stays interoperable with cosign, GitHub attestations, and `slsa-verifier`.

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    { "name": "patch.diff", "digest": { "sha256": "aaaa…aaaa" } },
    { "name": "repo-tree",  "digest": { "sha256": "bbbb…bbbb" } }
  ],
  "predicateType": "https://agentattest.dev/predicate/v0",
  "predicate": {
    "predicateVersion": "v0",
    "runId": "run_01HTYJ7B4A3H8M0S8P7M2N9Q",
    "verificationLevel": "policy-grade",
    "repo": {
      "url": "https://github.com/example/agentattest",
      "baseCommit": "0123…4567",
      "branch": "feature/agent-binding",
      "pullRequest": { "number": 123 }
    },
    "agent": {
      "name": "example-agent",
      "version": "1.0.0",
      "invocationKind": "ci-step",
      "declaredIdentity": "sigstore:github:example-agent"
    },
    "environment": {
      "executionType": "github-actions",
      "builder": {
        "id": "https://github.com/.../workflows/agentattest.yml@refs/heads/main",
        "type": "github-actions-workflow",
        "workflowRef": "...@refs/heads/main",
        "runnerClass": "github-hosted"
      }
    },
    "humanApproval": { "state": "approved-after-generation" },
    "privacy": {
      "redactionPolicy": "default-minimal-v0",
      "rawPrompt":      { "stored": false, "visibility": "none" },
      "rawToolOutputs": { "stored": false, "visibility": "none" },
      "publicTransparencyLog": { "allowed": true, "includesRawContent": false }
    },
    "timestamps": { "startedAt": "2026-04-27T09:12:31Z", "finishedAt": "2026-04-27T09:16:42Z" },
    "evidence": [
      {
        "type": "tool-summary",
        "uri": "artifact://...",
        "digest": { "sha256": "..." },
        "storage": "artifact-store",
        "visibility": "team"
      }
    ]
  }
}
```

### Required predicate fields

`predicateVersion` · `runId` · `verificationLevel` · `repo` · `agent` · `environment` · `humanApproval` · `privacy` · `timestamps` · `evidence`

### Optional

`model` · `runtime` (OTel / OpenInference trace IDs) · `changes` · `sidecars` (SPDX / CycloneDX / OpenVEX) · `materials` · `extensions` (URI-keyed, **closed**, digest-addressed only)

**v1 optional additions** · `capture` (how the run was captured — `harness-native` / `wrapper` / `ci-step` / `manual`) · `agentConfig` (digest of the `AGENTS.md` / rules that governed the run) · `mcpServers` + `tools` (MCP server identity + per-tool schema digests) · `delegation` (subagent / multi-agent chain)

The full schema is split across three files that **must stay in lockstep**:

| File | Owns |
|---|---|
| [`schemas/agent-provenance-v0.schema.json`](schemas/agent-provenance-v0.schema.json) | Structure, `additionalProperties: false`, types, URI patterns, GitHub builder string shape |
| [`schemas/agent-provenance-v0.cue`](schemas/agent-provenance-v0.cue) | Cross-field semantics — timestamp ordering, runtime ↔ trace-evidence binding, raw-visibility coupling |
| [`schemas/agent-provenance-v1.schema.json`](schemas/agent-provenance-v1.schema.json) | v1 structure — adds `agentConfig` / `mcpServers` / `tools` / `delegation` / `capture` / `platform-agent` builder |
| [`schemas/agent-provenance-v1.cue`](schemas/agent-provenance-v1.cue) | v1 cross-field semantics — `harness-native ⇒ bound trace`, plus every v0 rule |
| [`policies/default.rego`](policies/default.rego) | Runtime context — repo / base match, exact subject set, verified identity, level escalation, replay, freshness, and the v1 agentConfig / MCP / delegation gates |

---

## Verification Pipeline

Five ordered phases. Each invariant has **one** owning phase; later phases must not reclassify earlier-phase failures.

| Phase | Owner | Checks |
|---|---|---|
| 01 | Go pre-schema gate (`internal/verify/phase01`) | `_type`, `predicateType`, `predicate.predicateVersion` routing for **v0/v1** (with type↔version consistency) — *before* JSON Schema |
| 02 | JSON Schema 2020-12 (`santhosh-tekuri/jsonschema`) | required fields, closed objects, enums, URI safety, GitHub builder shape |
| 03 | CUE (`cuelang.org/go`) | cross-field semantics: timestamp ordering, `runtime ⇒ trace evidence` |
| 04 | OPA Rego (`open-policy-agent/opa`) | required level, exact subject set, repo / base match, verified signer / builder / workflow / issuer, level escalation, public-log privacy, witness / approval, replay, freshness |
| 05 | Go post-policy ordering (`OrderFailureCodes`) | dedupe + stable failure-code ordering for user-visible output |

Phase 04 is the **only** phase that emits `level_escalation`. JSON Schema and CUE intentionally allow a policy-grade predicate with `local-only` evidence so Rego can produce the stable code.

---

## Verification Levels

```
                    evidence-grade  ──►  policy-grade  ──►  high-assurance
                    ──────────────       ────────────       ──────────────
local execution     allowed              forbidden          forbidden
local-only ev.      allowed              forbidden          forbidden
verified signer     optional             required           required
verified builder    optional             required           required
isolated/witnessed  —                    —                  required
verified witness    —                    —                  ≥1 required
approved-before-
  merge + digest    —                    —                  required
runner allowlist    —                    —                  required
```

| Level | Intended for | Proves | Does not prove |
|---|---|---|---|
| `evidence-grade` | local CLI, dev workflows, early PR evidence | a statement was produced binding one run to subjects + privacy settings | local machine integrity, agent metadata truthfulness |
| `policy-grade` | CI / GitHub merge gates | signed / verified envelope, exact subject equality, verified non-local builder / signer / workflow / issuer | runner cannot be compromised, code is correct |
| `high-assurance` | isolated runners, witnessed execution, protected merges | all of policy-grade **+** allowlisted isolated/witnessed builder, ≥1 verified witness, verified `approved-before-merge` digest, freshness/replay context | hypervisor / firmware / SaaS plane integrity, freedom from vulnerabilities |

A level is a **policy profile**, not a universal trust proof. Local capture caps at evidence-grade; CI/GitHub reaches policy-grade; isolated runner + witness + protected merge gate is what approaches high-assurance.

---

## Failure Codes

All codes are **stable strings** — part of the public contract. The Go verifier emits them in the deterministic order below (defined in [`internal/verify/failure_order.go`](internal/verify/failure_order.go)). Rego deny sets are unordered, so Go owns presentation.

| # | Code | Meaning |
|---|---|---|
| 10  | `unsupported_statement_type` | `_type` is not in-toto Statement v1 |
| 20  | `predicate_type_mismatch` | `predicateType` is not `https://agentattest.dev/predicate/v0` |
| 30  | `unsupported_predicate_version` | `predicateVersion` is not `v0` (rejected before schema) |
| 40  | `schema_invalid` | JSON Schema or CUE failure |
| 50  | `signature_invalid` | DSSE / Sigstore envelope signature failed |
| 60  | `transparency_verification_failed` | Rekor / timestamp / witness verification failed when required |
| 70  | `subject_digest_mismatch` | statement subjects ≠ context subjects (set equality) |
| 80  | `repo_url_mismatch` | predicate repo URL ≠ context repo URL |
| 90  | `base_commit_mismatch` | predicate base commit ≠ context base commit |
| 100 | `signer_identity_mismatch` | verified signer missing or ≠ `agent.declaredIdentity` |
| 110 | `builder_identity_mismatch` | verified builder / workflow / issuer missing or unmatched; self-hosted without opt-in; high-assurance runner not allowlisted |
| 120 | `replay_detected` | PR number / `runId` differs from verifier context |
| 130 | `privacy_violation` | raw / trace evidence on transparency-log; non-public on transparency-log; public extension |
| 140 | `level_escalation` | policy-grade or high-assurance with local execution / `local-only` evidence / missing builder / missing high-assurance evidence |
| 150 | `level_below_required` | predicate level below `context.requiredLevel` |
| 160 | `missing_evidence` | required trace / approval / witness / digest reference absent |
| 170 | `stale_attestation` | outside the configured freshness window, or `now` missing while window > 0 |
| 180 | `policy_eval_error` | policy engine cannot evaluate deterministic inputs |
| 190 | `cache_untrusted` | SQLite cache row conflicts with verified statement; cache is ignored |

> **Multi-code ordering** is verified separately from golden fixtures: each invalid golden has *exactly one* expected code. The ordering fixture lives at [`internal/verify/testdata/unsupported-version-subject-mismatch.json`](internal/verify/testdata/unsupported-version-subject-mismatch.json).

---

## Privacy Defaults

Default rule: **do not store raw prompts, raw tool outputs, raw retrieved documents, or raw model responses.** Store digests, references, redacted summaries, and encrypted blobs only.

| Visibility | Use for |
|---|---|
| `public` | digests, predicate type, repo URL, subject names, non-sensitive summaries |
| `team` | redacted trace summaries, CI artifacts, approval metadata |
| `secret` | sensitive tool summaries, private issue refs, internal IDs |
| `encrypted` | raw prompts, raw tool outputs, private traces, customer data |

Hard structural rules, enforced jointly by JSON Schema, CUE, and Rego:

- `rawPrompt.visibility` and `rawToolOutputs.visibility` may **never** be `public`.
- If `stored == false` then `visibility` **must** be `none`.
- `evidence[].uri` rejects `data:` payloads and HTTP(S) `userinfo` (e.g. `https://token:x@host/...`).
- `extensions` is URI-keyed, **closed**, and every value is a digest-addressed reference object — no inline blobs, no `public` extensions.
- `publicTransparencyLog.includesRawContent` is constrained to `false`.

> Structural checks do not scan referenced artifact contents. Redaction is engineering hygiene, not a proof of safety.

See [`docs/PRIVACY_MODEL.md`](docs/PRIVACY_MODEL.md) and [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md).

---

## Project Layout

```
agentattest/
├── cmd/agentattest/                 CLI entry — 6 lines, routes to internal/app
├── internal/
│   ├── app/         orchestration   init · digest · predicate · verify
│   ├── gitbind/     deterministic   repo URL · base/head · patch sha256 · file sha256
│   ├── predicate/   v0/v1 predicate  Create/CreateV1 + StatementFor()/StatementForV1()
│   ├── statement/   in-toto v1      parse / new / typed Document
│   ├── verify/      pipeline        phase01..05, OrderFailureCodes, golden_test, testdata
│   ├── signing/     DSSE + X.509    Sign/Verify envelope → verified signer/builder/issuer context
│   ├── policy/      Rego adapter    wraps open-policy-agent/opa
│   ├── cache/       SQLite          refs · digests · timestamps  (no pass/fail)
│   └── contracts/   path locator    walks up to find schema/CUE/policy/context
├── schemas/
│   ├── agent-provenance-v0.schema.json     JSON Schema 2020-12 (v0)
│   ├── agent-provenance-v0.cue             CUE cross-field semantics (v0)
│   ├── agent-provenance-v1.schema.json     JSON Schema 2020-12 (v1, frontier-harness)
│   └── agent-provenance-v1.cue             CUE cross-field semantics (v1)
├── policies/
│   └── default.rego                         OPA Rego phase-04 policy
├── tests/golden/                            ~30 fixtures · context.schema.json
│   ├── valid-minimal/   valid-github-ci/   valid-high-assurance/
│   └── invalid-*/       (one expected failure code each)
├── docs/
│   ├── DATA_MODEL.md           predicate fields & semantics
│   ├── FRONTIER_HARNESS_2026.md 2026 harness landscape review + gap analysis
│   ├── VERIFICATION_MODEL.md   levels, phases, failure codes
│   ├── PRIVACY_MODEL.md        defaults, visibility classes
│   ├── THREAT_MODEL.md         threats, mitigations, residual risk
│   ├── NON_GOALS.md            hard scope boundaries
│   └── EXISTING_WHEELS.md      composition map
├── AGENTS.md                   entry map and strict rules
├── ARCHITECTURE.md             layering, module boundaries, forbidden imports
├── TASKS.md                    v0 → v1 → integration milestones & acceptance criteria
└── CLAUDE.md                   working notes for AI agents on this repo
```

---

## Existing Wheels

`agentattest` is a **composition layer**. Anything below is reused as-is and must not be re-implemented.

| Standard / tool | Role |
|---|---|
| **in-toto Statement v1** | Outer container `_type / subject / predicateType / predicate` |
| **DSSE** | Signing envelope for statements |
| **Sigstore / cosign / Fulcio / Rekor** | Keyless signing, OIDC certs, transparency log |
| **GitHub Artifact Attestations** · `actions/attest` · `gh attestation` | GitHub-native attestation storage and verification |
| **SLSA provenance vocabulary** | Builder, workflow, materials terminology |
| **OpenTelemetry / OpenInference** | Run-time trace transport and AI semantic conventions |
| **SPDX / CycloneDX / OpenVEX** | SBOM / BOM / VEX sidecars |
| **OPA / Rego** | Policy-as-code (phase 04) |
| **CUE** | Schema + cross-field constraints (phase 03) |
| **JSON Schema 2020-12** | Structural validation (phase 02) |
| **SQLite** (`modernc.org/sqlite`) | Local cache: refs, digests, timestamps only |
| **Git** | Source identity — commits, refs, normalized patch |

See [`docs/EXISTING_WHEELS.md`](docs/EXISTING_WHEELS.md).

---

## Roadmap

Tracked in [`TASKS.md`](TASKS.md) with deterministic acceptance criteria — *"no task may rely on AI judgment to decide whether it is secure."*

| Milestone | Scope | Status |
|---|---|---|
| **v0 foundation** | Skeleton · predicate types · git binding · cache · in-toto assembly · Rego policy · golden harness · privacy gate | **shipped** — 33 golden fixtures pass |
| **v0 signing** | `internal/signing`: DSSE envelope + X.509 identity + trust-root chain verification → verified verifier context | **shipped (envelope + identity + chain)**; Rekor inclusion-proof + Sigstore/`gh attestation` bundle ingestion pending |
| **v1 contract** | `agentConfig` · `mcpServers`/`tools` · `delegation` · `capture` · `platform-agent` — binds the 2026 harness plane | **shipped in this upgrade** — 8 new golden fixtures pass |
| **v0.1 integration** | GitHub Action · attestation verification · PR summary · harness capture SDK · registry | planned |

---

## Hard Invariants

These will trip you up if violated. They are enforced by code **and** by review.

- `predicateType` is **exactly** `https://agentattest.dev/predicate/v0`.
- `predicate.predicateVersion` is **exactly** `v0`. Unsupported versions are rejected in **phase 01**, before JSON Schema / CUE / Rego.
- Every predicate object has `additionalProperties: false`. `extensions` is URI-keyed but each value is a closed digest-addressed reference.
- A schema change without a matching CUE update **and** a golden fixture update is a broken change.
- Failure codes are stable strings; renaming them is a public-contract break.
- Raw prompt and raw tool-output visibility may **never** be `public`. If `stored == false`, visibility must be `none`.
- `local-only` evidence is allowed **only** at evidence-grade. Rego owns this and emits `level_escalation`.
- Subject digests are checked as **equal sets**, not supersets. Extra statement subjects are rejected.
- Verified envelope / certificate data outranks anything in the predicate. The cache never decides pass/fail.
- Only `internal/signing` may import DSSE / cosign / Fulcio / Rekor / GitHub-attestation libraries; only hashing / digest helpers may use `crypto/*` elsewhere.
- External commands use **argv arrays**, never shell-interpolated strings.

Breaking the predicate contract requires a **new predicate URI** and **new golden fixtures** — never in-place edits.

---

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for the full text.

---

## Contributing

Before any change:

1. [`AGENTS.md`](AGENTS.md) — entry map and strict rules.
2. [`ARCHITECTURE.md`](ARCHITECTURE.md) — module boundaries, allowed / forbidden imports.
3. The doc that owns what you're touching (see the table in [`CLAUDE.md`](CLAUDE.md)).
4. [`docs/NON_GOALS.md`](docs/NON_GOALS.md) — do **not** negotiate scope without revisiting this.

Predicate / schema / CUE / policy changes require:

- Schema **and** CUE updated together.
- At least one new or updated golden fixture in [`tests/golden/`](tests/golden/).
- Failure codes use the existing stable strings, or add a new one and document it in [`docs/VERIFICATION_MODEL.md`](docs/VERIFICATION_MODEL.md).
- `go test ./...` passes on Linux, macOS, and Windows.

---

<div align="center">

**Trust comes from verified envelope, signer / builder / workflow identity, and recomputed subject digests — not from anything self-asserted in the predicate.**

[English](./README.md) &nbsp;|&nbsp; [简体中文](./README.zh-CN.md)

</div>
