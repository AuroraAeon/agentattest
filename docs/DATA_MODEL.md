# Data Model

The v0 predicate type is:

```text
https://agentattest.dev/predicate/v0
```

`agentattest` uses in-toto Statement v1 as the outer container. The custom JSON Schema applies only to the `predicate` object, not to the complete statement.

## Outer Statement

Every v0 attestation is an in-toto Statement v1 with this shape:

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "patch.diff",
      "digest": {
        "sha256": "..."
      }
    }
  ],
  "predicateType": "https://agentattest.dev/predicate/v0",
  "predicate": {}
}
```

The `subject` array binds the signed claim to concrete artifacts. For v0, subjects should use SHA-256 digests over deterministic inputs:

| Subject Name | Meaning |
|---|---|
| `patch.diff` | Normalized patch produced or modified by the agent run |
| `repo-tree` | Deterministic tree snapshot or tree-manifest digest |
| `pr-<number>` | Pull request state digest when available |
| artifact name | Build output, container image, archive, binary, or other release artifact |

The verifier must compare statement subjects to locally computed or CI-provided current subjects as equal sets. Extra statement subjects are rejected because downstream consumers might otherwise trust a subject the verifier did not intend to check.

## Required Predicate Fields

| Field | Type | Requirement |
|---|---|---|
| `predicateVersion` | string | Must be `v0`; non-v0 values are rejected by the verifier pre-schema gate |
| `runId` | string | Stable ID for one agent run |
| `verificationLevel` | enum | `evidence-grade`, `policy-grade`, or `high-assurance` |
| `repo` | object | Repository URL and base commit binding |
| `agent` | object | Self-asserted agent metadata |
| `environment` | object | Capture and builder environment metadata |
| `humanApproval` | object | Human approval state and optional reviewer refs |
| `privacy` | object | Raw-evidence storage and redaction declarations |
| `timestamps` | object | Run start and finish timestamps |
| `evidence` | array | Digest-addressed evidence references |

## Optional Predicate Fields

| Field | Type | Purpose |
|---|---|---|
| `model` | object | Self-asserted model/provider metadata |
| `runtime` | object | OpenTelemetry/OpenInference trace identifiers; if present, trace evidence is required |
| `changes` | object | File count, insertion, deletion, and generated-file summary |
| `sidecars` | array | SPDX, CycloneDX, OpenVEX, or related sidecar document references |
| `materials` | array | Input materials such as issue refs, source refs, or dependency snapshots |
| `extensions` | object | URI-keyed extension references constrained to digest-addressed external content |

## Field Semantics

`repo.url` is the canonical repository URL expected by policy. It must be HTTP(S) without credentials in the URL. `repo.baseCommit` is the commit from which the agent work started. `repo.headCommit` is optional because local evidence can exist before commit creation.

`agent` and `model` fields are descriptive metadata. They do not establish trust by themselves. `agent.declaredIdentity` is low-trust unless it matches `context.verifiedSigner` supplied by the verifier from a verified envelope, certificate, CI identity, or trusted policy input.

`environment.executionType` identifies whether capture happened locally, in GitHub Actions, another CI system, an isolated runner, or a witnessed runner. Policy-grade and high-assurance claims require verified non-local builder identity from verifier context, not only the self-asserted `environment.builder` field.

`humanApproval.state` records approval state. It does not replace branch protection, CODEOWNERS, or platform review enforcement. High-assurance requires digest-addressed approval evidence plus `context.verifiedApproval=true` and `context.verifiedApprovalDigest` matching the approval evidence digest. Those context fields are derived out of band by the verifier.

`privacy.rawPrompt` and `privacy.rawToolOutputs` default to not stored. If raw evidence is explicitly stored, it must not be public and should be encrypted or access-controlled.

`evidence` entries are references, not embedded raw data. Each entry has a type, URI, SHA-256 digest, storage class, and visibility class. URI fields use an allowlist of reference schemes and reject `data:` payloads and HTTP(S) userinfo.

`runtime` contains trace identifiers only. When it is present, `evidence` must contain a `trace` entry whose digest binds an exported trace document. `agentattest` does not define or store a trace protocol.

`sidecars` link external documents. SPDX, CycloneDX, and OpenVEX remain separate documents with their own schemas.

`extensions` is a constrained extension point. Each URI-keyed value must be an object with `uri`, `digest`, `mediaType`, `visibility`, and optional `retention`. Inline raw blobs and unplanned fields are not allowed. The default policy rejects public extensions.

## v1 Additions (`https://agentattest.dev/predicate/v1`)

v1 is a superset of v0. Every v0 field, privacy invariant, and failure code is unchanged. v1 adds five optional, digest-addressed surfaces that bind the frontier coding-agent harness plane (reviewed in [`FRONTIER_HARNESS_2026.md`](./FRONTIER_HARNESS_2026.md)).

| Field | Type | Purpose |
|---|---|---|
| `capture` | object | How the run was captured: `method` ∈ `harness-native`/`wrapper`/`ci-step`/`manual`, plus `harness` (name) when `harness-native`. `harness-native` requires a `runtime` trace and a `trace` evidence entry. |
| `agentConfig` | object | The agent operating contract in effect: `primaryFile` (e.g. `AGENTS.md`), `primaryDigest`, and optional `files[]` (`{ path, digest, role }`, role ∈ `instructions`/`tool-rules`/`repo-policy`). |
| `mcpServers` | array | MCP server identity: `{ name, version?, transport, serverIdentity?, digest }` where `digest` covers the server's tool-manifest / schema bundle. |
| `tools` | array | Per-tool schema binding: `{ name, server?, schemaDigest }`. |
| `delegation` | object | Multi-agent / subagent chain: `delegationChain[]` of `{ role, agentRef, identity?, digest? }`. |

v1 also extends a few enums for first-party platform coding agents:

- `agent.invocationKind` gains `platform-agent`.
- `environment.executionType` gains `platform-agent`; `environment.builder.type` gains `platform-agent`; `builder.runnerClass` gains `platform-managed`.
- `evidence.type` gains `agent-config`.

These are **self-asserted** like `agent`/`model`. Trust still comes from verified signer / builder / workflow identity plus recomputed subject digests. The default policy enforces v1 semantics deterministically:

- **High-assurance v1** requires a bound `agentConfig` (fails with `missing_evidence`).
- When the verifier supplies `context.verifiedAgentConfig`, the declared `agentConfig.primaryDigest` must match it (`missing_evidence`).
- When the verifier supplies `context.verifiedMcpServers` (an allowlist), every declared `mcpServers[].digest` must appear in it (`missing_evidence`).
- When the verifier supplies `context.verifiedDelegation`, the `delegationChain` root `agentRef` must appear in it (`missing_evidence`).

No raw prompts, tool outputs, or trace payloads are ever stored; MCP / tool binding is digest-only. See [`PRIVACY_MODEL.md`](./PRIVACY_MODEL.md).

## Sample JSON

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "name": "patch.diff",
      "digest": {
        "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      }
    },
    {
      "name": "repo-tree",
      "digest": {
        "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    }
  ],
  "predicateType": "https://agentattest.dev/predicate/v0",
  "predicate": {
    "predicateVersion": "v0",
    "runId": "run_01HTYJ7B4A3H8M0S8P7M2N9Q",
    "verificationLevel": "policy-grade",
    "repo": {
      "url": "https://github.com/example/agentattest",
      "baseCommit": "0123456789abcdef0123456789abcdef01234567",
      "headCommit": "89abcdef0123456789abcdef0123456789abcdef",
      "branch": "feature/agent-binding",
      "pullRequest": {
        "number": 123,
        "url": "https://github.com/example/agentattest/pull/123"
      }
    },
    "agent": {
      "name": "example-coding-agent",
      "version": "1.2.3",
      "invocationKind": "ci-step",
      "declaredIdentity": "sigstore:github:example-agent"
    },
    "environment": {
      "executionType": "github-actions",
      "builder": {
        "id": "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main",
        "type": "github-actions-workflow",
        "workflowRef": "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main",
        "runnerClass": "github-hosted"
      }
    },
    "runtime": {
      "traceFormat": "openinference-otel-json",
      "traceIds": [
        "4bf92f3577b34da6a3ce929d0e0e4736"
      ]
    },
    "humanApproval": {
      "state": "approved-after-generation",
      "reviewerRefs": [
        "github:user:maintainerA"
      ]
    },
    "privacy": {
      "redactionPolicy": "default-minimal-v0",
      "rawPrompt": {
        "stored": false,
        "visibility": "none"
      },
      "rawToolOutputs": {
        "stored": false,
        "visibility": "none"
      },
      "publicTransparencyLog": {
        "allowed": true,
        "includesRawContent": false
      }
    },
    "timestamps": {
      "startedAt": "2026-04-27T09:12:31Z",
      "finishedAt": "2026-04-27T09:16:42Z"
    },
    "evidence": [
      {
        "type": "trace",
        "uri": "artifact://agentattest/traces/run_01HTYJ7B4A3H8M0S8P7M2N9Q.otel.json",
        "digest": {
          "sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
        },
        "mediaType": "application/json",
        "storage": "artifact-store",
        "visibility": "team"
      }
    ],
    "sidecars": [
      {
        "type": "spdx",
        "uri": "artifact://agentattest/sbom.spdx.json",
        "digest": {
          "sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
        },
        "mediaType": "application/spdx+json",
        "visibility": "team"
      }
    ]
  }
}
```
