# Tasks

Implementation is split into v0-alpha, v0-beta, and v0. Every task has deterministic acceptance criteria. No task may rely on AI judgment to decide whether it is secure.

## v0-alpha

### 1. Repository Skeleton

Create the Go module, CLI entry point, internal package layout, schema directories, policy directories, and test directories.

Acceptance criteria:

- `go test ./...` runs.
- Package layout follows `ARCHITECTURE.md`.
- No package imports violate allowed dependency directions.
- CI runs formatting, tests, JSON Schema 2020-12 validation, `cue vet`, `opa eval`, and the Go golden fixture runner.

### 2. Predicate Types And Validation

Implement Go types and validation for `https://agentattest.dev/predicate/v0`.

Acceptance criteria:

- Valid minimal and valid GitHub CI golden predicates pass JSON Schema validation.
- Invalid golden predicates fail with stable failure codes.
- Go validation rejects unsupported `predicateVersion` before JSON Schema, CUE, or policy evaluation.
- Tests cover required fields, `additionalProperties` rejection, constrained extensions, URI scheme/userinfo rejection, runtime trace evidence binding, and context schema validation.

### 3. Deterministic Git Binding

Compute current repo URL, base commit, normalized patch digest, and tree or tree-manifest digest.

Acceptance criteria:

- Same input patch produces the same SHA-256 digest on Windows, macOS, and Linux.
- Subject mismatch test fails deterministically.
- Base commit mismatch test fails deterministically.
- No signing or policy code is imported by the git binding package.

### 4. Local Evidence Capture Contract

Capture minimal run metadata and evidence references without raw prompts or raw tool outputs by default.

Acceptance criteria:

- Default output has `rawPrompt.stored=false` and `rawToolOutputs.stored=false`.
- Evidence entries include URI, SHA-256 digest, storage, and visibility.
- Raw prompt capture requires explicit opt-in.
- Public raw prompt evidence is rejected by validation or policy.

### 5. SQLite Cache Prototype

Index run IDs, subject digests, statement paths, evidence references, content digests, and last-seen timestamps in SQLite.

Acceptance criteria:

- Verification can run without trusting cache contents.
- Cache corruption does not produce a valid verification result.
- Cache tables do not store authoritative pass/fail decisions.
- Schema migrations are deterministic and tested.
- No sensitive raw prompt/tool output is stored in default cache tables.

## v0-beta

### 6. in-toto Statement Assembly

Wrap the predicate in in-toto Statement v1.

Acceptance criteria:

- `_type` is exactly `https://in-toto.io/Statement/v1`.
- `predicateType` is exactly `https://agentattest.dev/predicate/v0`.
- Statement subjects are derived from deterministic digest inputs.
- Predicate schema validates before statement output.

### 7. DSSE And Sigstore Adapter

Add signing and verification adapters through existing DSSE/Sigstore/cosign-compatible libraries or commands.

Acceptance criteria:

- Signing code is isolated to `internal/signing`.
- No custom signing primitive is implemented.
- Verification rejects tampered statement payloads.
- Verification exposes signer, certificate, issuer, workflow, builder, witness, approval, and timestamp data as structured verifier input.

### 8. Default Rego Policy Integration

Evaluate `policies/default.rego` against verified statement and context.

Acceptance criteria:

- Repo URL mismatch fails.
- Base commit mismatch fails.
- Subject digest mismatch fails.
- Invalid verification level fails.
- `context.requiredLevel` is enforced with `level_below_required`.
- Statement subjects are enforced as equal sets, not supersets.
- Policy-grade and high-assurance require verified signer, builder, workflow, and issuer context.
- High-assurance requires verified witness and approval context.
- Public raw prompt evidence fails.
- Policy-grade with `local-only` evidence fails.

### 9. Golden Test Harness

Implement golden fixture runner for schema and policy tests.

Acceptance criteria:

- All required fixture categories in `tests/golden/README.md` exist.
- Fixture results are stable across repeated runs.
- Failure codes are stable strings.
- Go-side tests enforce stable failure-code ordering, including version failure before subject mismatch.
- Multi-code ordering tests are separate from golden fixtures; each invalid golden fixture has exactly one expected failure code.
- Adding a new required predicate field without fixture updates fails CI.

### 10. Privacy Gate

Implement field-level privacy checks before signing or uploading attestations.

Acceptance criteria:

- Public transparency-log output is blocked when raw content, raw evidence references, trace evidence, public extensions, inline extension blobs, credentialed URLs, or `data:` URIs are present.
- Raw prompt visibility `public` is impossible through normal APIs and rejected if manually supplied.
- Redaction policy name is required.
- Tests cover raw prompt, raw tool output, and public log cases.

## v0

### 11. GitHub Action

Provide a GitHub Action using the Go binary or a composite wrapper.

Acceptance criteria:

- Action can generate a custom predicate attestation for a patch/tree/artifact subject.
- Action uses GitHub OIDC/Sigstore-compatible signing through supported tooling.
- Action can run the verifier and emit a check summary.
- Action fails closed on subject, repo, base commit, schema, privacy, or policy failure.

### 12. GitHub Attestation Verification Path

Integrate with GitHub Artifact Attestations and `gh attestation` where available.

Acceptance criteria:

- Verifier can consume downloaded GitHub attestation verification JSON.
- Policy can check expected repository and signer workflow.
- Custom predicate type verification is supported.
- Tests distinguish verified certificate/workflow data from user-controlled predicate data.

### 13. PR Summary Output

Generate a concise PR/check summary.

Acceptance criteria:

- Summary includes verification result, level, subject digest prefixes, repo/base status, signer/workflow status, and privacy status.
- Summary never includes raw prompts, raw tool outputs, raw traces, secrets, or private sidecar contents.
- Summary has deterministic output for golden inputs.
- Failing summaries include stable failure codes.

### 14. Release And Compatibility Contract

Publish v0 contract docs, schemas, policy, and examples.

Acceptance criteria:

- `AGENTS.md`, `ARCHITECTURE.md`, `docs/*`, `schemas/*`, `policies/*`, and `tests/golden/*` are consistent.
- JSON Schema and CUE constraints agree for all golden fixtures.
- The README or release notes state non-goals and privacy defaults.
- A v0 compatibility test suite is part of CI.

### 15. Security Review Checklist

Add deterministic security review checks for v0.

Acceptance criteria:

- No custom crypto implementation exists.
- No package outside signing adapters performs signing.
- No default raw prompt or raw tool-output storage exists.
- No public-log path accepts raw content.
- Policy-grade and high-assurance reject local-only evidence.
- Verification failure modes are documented and tested.
