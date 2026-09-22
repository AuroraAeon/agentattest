# agentattest GitHub Action

A composite action that binds an AI coding-agent run to the current change and
verifies it against repository policy. It wraps the `agentattest` CLI — it never
re-implements the verifier — and **fails closed** on any failure code.

## What it does

1. Checks out the repo and builds the `agentattest` CLI.
2. Builds a verifier context from the current repo (`agentattest context`), or
   uses a `context` input you supply.
3. Creates a predicate (`agentattest predicate create --version …`).
4. Runs the 5-phase verifier (`agentattest verify predicate`). A non-zero exit
   fails the job.
5. Posts a deterministic, privacy-safe summary to `$GITHUB_STEP_SUMMARY`
   (`agentattest summary`).

## Usage

From the same repository (local action):

```yaml
# .github/workflows/agentattest.yml
name: agentattest
on: [pull_request]
permissions:
  contents: read
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: ./action
        with:
          version: v1
          required-level: evidence-grade
          agent-name: my-coding-agent
          capture-method: wrapper
          agent-config: AGENTS.md
```

From another repository (after publishing):

```yaml
      - uses: AuroraAeon/agentattest/action@v1
        with:
          required-level: policy-grade
```

## Inputs

| Input | Default | Purpose |
|---|---|---|
| `version` | `v1` | Predicate contract version (`v0` or `v1`). |
| `required-level` | `evidence-grade` | Minimum level the gate enforces. |
| `agent-name` / `agent-version` | — | Recorded in the predicate. |
| `capture-method` | `wrapper` | v1: `wrapper` \| `ci-step` \| `manual`. |
| `agent-config` | — | v1: path (e.g. `AGENTS.md`) bound by digest. |
| `model-provider` / `model-id` | — | v1 model identity (supply together). |
| `context` | — | Path to a verifier-context JSON to use instead of the generated one. |

## Outputs

- `valid` — `true`/`false`.
- `level` — verification level reached.

## Reaching policy-grade

Out of the box the action verifies at **evidence-grade** (subject/repo/base
binding + privacy). To gate at **policy-grade** or **high-assurance**, supply a
`context` JSON that carries verified identity
(`verifiedSigner` / `verifiedBuilderId` / `verifiedWorkflowRef` / `verifiedIssuer`).
Those fields are derived out of band from a verified envelope — for GitHub, from
`gh attestation verify` or a Sigstore/cosign keyless bundle. Ingesting that output
into the context is the documented next integration layer (`internal/signing`).

The action stores no secrets and never writes raw prompts, tool outputs, or traces
to the summary or logs.
