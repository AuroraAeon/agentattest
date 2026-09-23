# Security Review

Evidence for the TASKS.md "Security review checklist" (task 21). Every claim below is backed by code, tests, or fixtures in this repository. Last reviewed for v0.1.0.

## Checklist

### 1. No custom cryptography

**Holds.** All digest, signature, and proof math comes from Go's standard library or vetted libraries:

| Primitive | Source |
|---|---|
| SHA-256 digests (patch, tree, files, evidence, configs) | `crypto/sha256` |
| DSSE envelope + PAE | `secure-systems-lab/go-securesystemslib/dsse` (via `internal/signing`) |
| ECDSA / Ed25519 / RSA sign + verify | `crypto/*` via caller-supplied `crypto.Signer` |
| X.509 chain verification | `crypto/x509` (`leaf.Verify`) |
| RFC 6962 Merkle inclusion proofs | `transparency-dev/merkle` (`rfc6962.DefaultHasher` + `proof.VerifyInclusion`) |
| TUF metadata/target verification | `sigstore-go/pkg/tuf` (embedded, pinned root metadata) |

No hand-written hash constructions, HMACs, KDFs, or curve arithmetic exist. A previously hand-rolled RFC 6962 hasher was replaced with the library's `DefaultHasher` in v0.1.0. The import boundary is machine-enforced by `internal/signing/boundary_test.go`.

### 2. Signing only in `internal/signing`

**Holds, machine-enforced.** `internal/signing/boundary_test.go` walks every `.go` file in the module and fails the build if DSSE, Merkle, X.509, PEM, or asymmetric-crypto imports appear outside `internal/signing`. Other packages use `crypto/sha256` for digests only (allowed by `ARCHITECTURE.md`).

### 3. No default raw storage

**Holds.** Defaults proven by tests:

- `internal/predicate/create.go` defaults `rawPrompt`/`rawToolOutputs` to `stored=false, visibility="none"` (`create_test.go`).
- `internal/capture` has no raw fields in `RunMetadata` and only writes digest/reference evidence (`capture_test.go`).
- `internal/cache` stores refs, digests, and timestamps only — never pass/fail, never raw content.
- `internal/registry` stores subject digests and statement paths, never statement bytes.
- Raw capture is opt-in (explicit `stored=true` with `team`/`secret`/`encrypted` + `encryptedRef`) and can never be `public`.

### 4. No public-log raw content

**Holds, three layers.** JSON Schema (`includesRawContent: false` literal, safe-URI patterns, raw-evidence visibility coupling, closed objects), CUE (same invariants plus cross-field rules), and Rego:

- transparency-log evidence is restricted to an allowlist of metadata-shaped types (`local-log-root`, `builder-record`, `agent-config`, `mcp-server`); `trace`, `tool-summary`, `test-result`, `approval`, and raw refs are denied (`privacy_violation`).
- transparency-log evidence must be `public` (`privacy_violation`).
- extensions must never be `public` (`privacy_violation`).

Fixtures: `invalid-trace-on-transparency-log`, `invalid-tool-summary-on-transparency-log`, `invalid-raw-prompt-leakage`, `invalid-extensions-leak`, `invalid-data-uri-evidence`, `invalid-extensions-inline-blob`, `invalid-material-data-uri`, `invalid-credentials-in-repo-url`.

Known limit (documented in `PRIVACY_MODEL.md`): checks are structural, not content-scanning — referenced artifacts are not opened.

### 5. Policy-grade / high-assurance reject local-only evidence

**Holds.** `policies/default.rego` denies `local-only` evidence and `executionType: local` at policy-grade and above (`level_escalation`), plus missing builder identity. Fixture: `invalid-level-escalation`.

### 6. v1 gates fail closed

**Holds as of v0.1.0** (fail-open gaps found in the v0.1.0 review were closed):

| Gate | Behavior |
|---|---|
| Version routing | `unsupported_statement_type` / `predicate_type_mismatch` / `unsupported_predicate_version` before schema, CUE, or policy |
| `agentConfig` | digest mismatch rejected whenever `verifiedAgentConfig` is supplied; required at high-assurance |
| `mcpServers` | every declared digest must be allowlisted; declaring servers at policy-grade+ without a verified allowlist fails closed |
| `delegation` | every chain step must be in `verifiedDelegation`; declaring a chain at policy-grade+ without a verified allowlist fails closed |
| `capture` | `harness-native` structurally requires `runtime` + trace evidence (CUE); high-assurance requires trace evidence and rejects `manual` |
| Contract files | missing v1 contract files fail closed with `schema_invalid` |

Fixtures: `invalid-v1-version-type-mismatch`, `invalid-v1-capture-native-without-trace`, `invalid-v1-agent-config-mismatch`, `invalid-v1-mcp-not-allowlisted`, `invalid-v1-mcp-unverified`, `invalid-v1-delegation-unverified`, `invalid-v1-delegation-unverified-step`, `invalid-high-assurance-no-trace`, `invalid-high-assurance-manual-capture`.

### 7. Failure modes documented and tested

**Holds.** Every failure code emitted by the verifier has a stable string, a documented meaning in `docs/VERIFICATION_MODEL.md`, and deterministic ordering via `internal/verify/failure_order.go` (ordering pinned by `TestFailureOrderingFixture`). Ghost codes (`cache_untrusted`) were removed; `signature_invalid` is now emitted as a structured result by `verify bundle`. Codes without golden fixtures (`repo_url_mismatch`, `base_commit_mismatch`, `policy_eval_error`, `unsupported_statement_type`, `predicate_type_mismatch`) are covered by Go tests — fixtures added for the first three in v0.1.0.

## Known limitations (accepted, documented)

- No content scanning: referenced evidence artifacts are never opened; structural privacy only.
- Self-asserted fields (`agent`, `model`, `mcpServers[].name`, `delegation` structure) are claims unless backed by verified context.
- The verifier does not prove runner integrity, code quality, or model correctness (see `docs/NON_GOALS.md`).
- `tools[].schemaDigest` is bound in the predicate but not policy-checked (no `verifiedTools` context exists).
