# Privacy Model

`agentattest` is designed for provenance with minimal disclosure. The default is no raw prompt storage and no raw tool-output storage.

## Default Rule

By default:

- Do not store raw prompts.
- Do not store raw tool outputs.
- Do not store raw retrieved documents.
- Do not store raw model responses.
- Do not store secrets, API responses, issue bodies, emails, customer text, or internal tickets unless explicitly opted in.
- Store structured metadata, digests, redacted summaries, and encrypted or access-controlled evidence references instead.

## v1 Note

The v1 additions (`agentConfig`, `mcpServers`, `tools`, `delegation`, `capture`) carry **no raw content** — only paths, names, versions, transports, identities, and SHA-256 digests. They inherit every visibility and public-log rule above: nothing they add may be `public` on a transparency log, and they never embed prompts, tool outputs, or trace payloads. `agentConfig.files[].path`, `mcpServers[].name`, and `tools[].name` are constrained non-PII patterns, so the operating-contract and tool surface can be disclosed as digests without leaking source or secrets.

## Field-Level Visibility

Every evidence-bearing field should be classified.

| Visibility | Meaning | Allowed For |
|---|---|---|
| `public` | Safe to publish in PR comments, release notes, and transparency-related metadata | digests, predicate type, repo URL, subject names, non-sensitive summary fields |
| `team` | Visible to repository or organization members | redacted trace summaries, CI artifacts, approval metadata, non-public sidecars |
| `secret` | Requires restricted access and should not appear in shared artifacts | sensitive tool summaries, private issue refs, internal system identifiers |
| `encrypted` | Stored only as encrypted blob references with external key control | raw prompts, raw tool outputs, private traces, customer data |

The raw prompt visibility class must be `none`, `team`, `secret`, or `encrypted`; it must never be `public`. If `stored` is `false`, visibility must be `none`.

## Redaction Policy

The default redaction policy is `default-minimal-v0`.

It should remove or replace:

- API keys, tokens, passwords, private keys, and session cookies.
- Email addresses, phone numbers, IP addresses, and other personal identifiers when not required for verification.
- Customer names, tenant IDs, ticket contents, and private issue text.
- Raw source snippets not necessary for subject binding.
- Raw tool responses that may contain prompt injections, secrets, or private documents.

Redaction is not a proof of safety. v0 structural privacy checks do not scan referenced evidence contents. If raw evidence must be retained, store it as encrypted evidence with limited retention and access controls.

## Transparency-Log Safety

Public transparency logs are appropriate for:

- Signatures.
- Certificates.
- Timestamps.
- Artifact and statement digests.
- Predicate type identifiers.
- Statement payloads only when the predicate is structurally safe to disclose.

Public transparency logs are not appropriate for:

- Raw prompts.
- Raw tool outputs.
- Raw traces containing messages or retrieved documents.
- Secrets or credentials.
- Customer data.
- Personal data that conflicts with deletion, correction, or minimization obligations.
- Private source code excerpts beyond digest-addressed subjects.

For v0 and v1, a predicate is structurally safe for public-log payloads only if all of these hold:

- No evidence type outside the transparency-log allowlist uses `storage: "transparency-log"`. The allowlist is metadata-shaped entries only: `local-log-root`, `builder-record`, `agent-config`, `mcp-server`. Everything else — `trace`, `tool-summary`, `test-result`, `approval`, `raw-prompt-ref`, `raw-tool-output-ref` — is denied by default, because those types can carry prompt, tool, trace, review, or test content.
- Any evidence with `storage: "transparency-log"` has `visibility: "public"`.
- `privacy.publicTransparencyLog.includesRawContent` is `false`.
- `extensions` is absent or every extension value is a constrained digest-addressed reference with non-public visibility.
- `agent.declaredIdentity`, `humanApproval.reviewerRefs[]`, `mcpServers[].name`, and `tools[].name` match the constrained non-PII patterns.
- No URI field uses `data:` or HTTP(S) userinfo.

These checks are enforced at two layers, and the emitted failure code differs by layer:

| Check | Layer | Code on failure |
|---|---|---|
| transparency-log type allowlist; transparency-log evidence must be `public`; extensions must not be public | Rego (`policies/default.rego`) | `privacy_violation` |
| `includesRawContent` is the literal `false`; raw-evidence visibility coupling; `data:`/userinfo URI rejection; non-PII name/identity/reviewer patterns; inline extension blobs | JSON Schema + CUE | `schema_invalid` |

The verifier computes these structural checks and fails closed when they fail. It does not scan the referenced artifact contents, so it does not prove those artifacts are free of secrets — only that no raw or content-shaped evidence is *structurally* placed on a public log.

## GDPR-Oriented Minimization

The project follows a minimization principle:

- Collect only fields needed to bind an agent run to subjects and policy.
- Prefer digests, references, and summaries over raw content.
- Keep raw evidence opt-in.
- Make retention explicit for encrypted evidence blobs.
- Avoid public immutable storage for personal data.
- Keep verification possible without exposing unnecessary personal or confidential data.

This is not legal advice. It is an engineering constraint intended to reduce privacy risk by default.

## Public Output Rules

PR comments, check summaries, and release summaries should show:

- Verification result.
- Verification level.
- Subject names and digest prefixes.
- Repo/base/PR match status.
- Signer or workflow identity when verified.
- Redaction status.
- Links to access-controlled evidence when available.

They should not show:

- Raw prompt text.
- Raw tool outputs.
- Raw model responses.
- Full private trace payloads.
- Secrets or suspected secrets.
- Sensitive sidecar content.
