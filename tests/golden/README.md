# Golden Fixtures

Golden fixtures define compatibility for schema validation, CUE validation, default policy behavior, and Go-side failure-code ordering. Every predicate schema, CUE, policy, or verifier-context change must add or update fixtures.

Golden fixtures are normal verifier cases and each invalid fixture must name exactly one expected failure code. Multi-code ordering cases live outside `tests/golden/` under Go testdata.

## Directory Structure

Each fixture directory uses the same three-file shape:

```text
tests/golden/
  <fixture-name>/
    statement.json
    context.json
    want.json
```

## File Roles

`statement.json` is a full in-toto Statement v1 with:

- `_type`.
- `subject`.
- `predicateType`.
- `predicate`.

`context.json` is verifier context that is not trusted from the predicate. It must validate against `tests/golden/context.schema.json`:

- `repoUrl`.
- `baseCommit`.
- `requiredLevel`.
- `subjects`, each with `name`, `algorithm`, and `digest`.
- Optional verified signer, builder, workflow, issuer, witness, approval digest, PR, run ID, runner allowlist, self-hosted runner, and freshness inputs.

`want.json` is the expected verifier result:

- `valid`: boolean.
- `level`: expected verification level when valid.
- `failureCodes`: expected deterministic failure codes when invalid.

## Required Fixtures

### `valid-minimal`

Evidence-grade predicate with:

- One `patch.diff` subject.
- Matching `context.subjects`.
- Local execution.
- `local-only` evidence.
- `rawPrompt.stored=false`.
- `rawToolOutputs.stored=false`.

Expected result: valid evidence-grade.

### `valid-github-ci`

Policy-grade predicate with:

- `patch.diff` and `repo-tree` subjects.
- Matching repo URL and base commit.
- `github-actions` execution.
- Builder identity present.
- No `local-only` evidence.
- Public transparency log allowed but `includesRawContent=false`.

Expected result: valid policy-grade.

### `valid-high-assurance`

High-assurance predicate with non-local isolated execution, verified signer/builder/workflow/issuer context, allowed isolated runner, verified witness context, trace evidence, approval evidence, `verifiedApproval=true`, and `verifiedApprovalDigest` matching the approval evidence digest.

Expected result: valid high-assurance.

### `invalid-replay`

Statement is otherwise well-formed but `context.expectedPullRequestNumber` or `context.expectedRunId` differs from the predicate.

Expected result: invalid with `replay_detected`.

### `invalid-subject-mismatch`

Statement subject digest differs from `context.subjects`.

Expected result: invalid with `subject_digest_mismatch`.

### `invalid-raw-prompt-leakage`

Predicate attempts to expose raw prompt data publicly, such as:

- Raw prompt evidence with `storage=transparency-log`.
- `publicTransparencyLog.includesRawContent=true` in schema-invalid variants.
- Any future raw-prompt public visibility path that bypasses schema constraints.

Expected result: invalid with `privacy_violation`.

### `invalid-level-escalation`

Predicate claims policy-grade or high-assurance but violates a level-owned policy gate such as `local-only` evidence, local execution, missing builder identity, or missing high-assurance witness.

Expected result: invalid with `level_escalation`.

### Additional Required Invalid Fixtures

| Fixture | Expected code |
|---|---|
| `invalid-level-downgrade` | `level_below_required` |
| `invalid-extensions-leak` | `privacy_violation` |
| `invalid-extensions-inline-blob` | `schema_invalid` |
| `invalid-data-uri-evidence` | `schema_invalid` |
| `invalid-credentials-in-repo-url` | `schema_invalid` |
| `invalid-extra-statement-subject` | `subject_digest_mismatch` |
| `invalid-self-asserted-isolated-runner` | `builder_identity_mismatch` |
| `invalid-witness-without-bundle` | `transparency_verification_failed` |
| `invalid-approval-unverified` | `missing_evidence` |
| `invalid-approval-digest-mismatch` | `missing_evidence` |
| `invalid-trace-id-without-evidence` | `schema_invalid` |
| `invalid-stale-attestation` | `stale_attestation` |
| `invalid-stale-missing-now` | `stale_attestation` |
| `invalid-builder-mismatch` | `builder_identity_mismatch` |
| `invalid-signer-mismatch` | `signer_identity_mismatch` |
| `invalid-trace-on-transparency-log` | `privacy_violation` |
| `invalid-additional-properties-conditional` | `schema_invalid` |
| `invalid-agent-identity-pii` | `schema_invalid` |
| `invalid-branch-name` | `schema_invalid` |
| `invalid-github-builder-id` | `schema_invalid` |
| `invalid-material-data-uri` | `schema_invalid` |
| `invalid-reviewer-ref-pii` | `schema_invalid` |
| `invalid-timestamp-order` | `schema_invalid` |

## v1 Fixtures

v1 fixtures (`predicateType` `https://agentattest.dev/predicate/v1`) exercise the frontier-harness additions. They use the same three-file shape and the one-code rule.

### Valid

| Fixture | Level | Exercises |
|---|---|---|
| `valid-v1-harness-native` | policy-grade | `capture: harness-native` + bound trace, `agentConfig`, `mcpServers`/`tools`, `delegation`, all matched to verified context |
| `valid-v1-platform-agent` | policy-grade | `executionType`/`builder.type: platform-agent` with a verified platform-bot signer; proves policy-grade does not require `agentConfig` |
| `valid-v1-high-assurance` | high-assurance | isolated runner + witness + approval + bound `agentConfig` (required at high-assurance v1) |

### Invalid (one code each)

| Fixture | Expected code |
|---|---|
| `invalid-v1-agent-config-mismatch` | `missing_evidence` |
| `invalid-v1-mcp-not-allowlisted` | `missing_evidence` |
| `invalid-v1-delegation-unverified` | `missing_evidence` |
| `invalid-v1-capture-native-without-trace` | `schema_invalid` |
| `invalid-v1-version-type-mismatch` | `unsupported_predicate_version` |

## Determinism Requirements

- Fixtures must not depend on wall-clock time unless the time is fixed in `context.json`.
- Digests must be lowercase hex.
- Subject names must be stable.
- Invalid fixtures should isolate one primary failure where practical.
- Golden output must use stable failure codes, not human-only error text.
- Invalid fixtures must pin exactly one expected code in `want.json`.
- Go-side ordering fixtures that intentionally trigger multiple codes must live outside `tests/golden/`; the current ordering fixture is `internal/verify/testdata/unsupported-version-subject-mismatch.json`.
