# Frontier Coding-Agent Harness Review — September 2026

> Purpose: re-ground `agentattest` in the *current* AI coding-agent harness ecosystem, five
> months after the original `deep-research-report.md` (April 2026). That report correctly
> argued for a **thin composition layer** over in-toto / DSSE / Sigstore / OpenTelemetry. That
> architectural bet has held up and is **preserved**. What has changed is the *harness plane*
> the layer sits on: several conventions that were "emerging" in April are now load-bearing
> de-facto standards, and they expose concrete gaps in the v0 predicate and roadmap.

This document is a review, not a redesign mandate. Every implication below is mapped to a
concrete, deterministic predicate field or policy input in the **v1** contract
(`https://agentattest.dev/predicate/v1`). Nothing here invents cryptography, containers, trace
protocols, or SBOM formats — see [`NON_GOALS.md`](./NON_GOALS.md).

---

## 1. What changed since April 2026 (the short version)

In April 2026 the ecosystem was fragmented: each coding agent shipped its own config file, its
own tool-calling conventions, and provenance was either platform-proprietary or absent. Five
months later, four things have consolidated into de-facto standards that a provenance layer
**must** model as first-class, verifiable inputs:

| # | Standard / shift | Status in Sept 2026 | Why it matters to `agentattest` |
|---|---|---|---|
| 1 | **`AGENTS.md`** as the cross-vendor agent-config contract | Open standard with a multi-vendor steering committee (OpenAI, Google, Sourcegraph/Amp, Cursor, Factory, Devin and others). Read at repo root by essentially every serious harness. | The agent's *operating contract* — build/test commands, code style, boundaries — is now a concrete, digestable repo artifact. It should be **attested**, not assumed. |
| 2 | **MCP (Model Context Protocol)** as the tool/data plane | Open protocol governed under the Linux Foundation's agentic-AI foundation; official server registry; server identity + JSON-schema tool definitions are normative. | Tool identity and **tool-schema digests** are the new supply-chain surface (tool poisoning is a named OWASP threat). v0 only had a single `mcp-server` evidence *type*. |
| 3 | **Harness-native telemetry** (OTel GenAI conventions + hooks) | OpenTelemetry GenAI semantic conventions are stable; leading harnesses (e.g. Claude Code) export OTel traces and expose lifecycle hooks natively. | Provenance can be captured **by the harness itself**, not only by wrapping the CLI. The *how captured* fact is now meaningful and should be attested. |
| 4 | **First-party platform coding agents** | GitHub Copilot coding agent (assign an issue → it opens a PR and commits as `copilot-swe-agent[bot]`), Google Jules, Codex cloud, Cursor background agents. | "The agent *is* the CI." Verified signer/builder identity now frequently belongs to a **platform bot**, not a human or a repo workflow. The identity vocabulary must recognize this. |

A fifth, quieter shift — **multi-agent / subagent orchestration** (subagents, planner/executor
splits, agent-to-agent delegation) — is now routine enough that a single flat "one agent made
this diff" claim under-describes real runs.

---

## 2. The harness plane, categorized

This is the "who actually produces agent-authored PRs today" map. It is intentionally
harness-agnostic: `agentattest` verifies claims, it does not integrate each product.

### 2.1 Local / CLI agents
Claude Code, OpenAI Codex CLI, Google Gemini CLI, Aider, opencode, Cline/Roo (terminal mode).

- Emit diffs against a working tree; often run on a developer machine.
- Config surface: `AGENTS.md` at repo root, plus per-tool files (`CLAUDE.md`, `.cursor/rules`,
  `.github/copilot-instructions.md`, `.cursorignore`, etc.).
- v0 mapping: `evidence-grade`, `executionType: local`, `local-only` evidence allowed. **Sound.**

### 2.2 IDE-embedded agents
Cursor (incl. background agents + Bugbot), Windsurf, GitHub Copilot (agent/chat mode), Continue.

- Config surface: `AGENTS.md` + tool-specific rule files.
- v0 mapping: `evidence-grade` locally; `policy-grade` if the IDE/CI signs. **Sound, but the
  config-file binding (AGENTS.md + rules) is unattested.**

### 2.3 Cloud / asynchronous agents (the fastest-moving category)
GitHub Copilot coding agent, Devin, Google Jules, OpenAI Codex cloud, Cursor background agents,
Factory "droids".

- These run in a **provider-controlled environment** and open PRs as a **platform bot**.
- The natural verifier context is a GitHub OIDC identity + a bot signer, *not* a repo workflow.
- v0 gap: the model assumes `environment.builder` is a **repo-owned GitHub Actions workflow**
  (the `#GitHubWorkflowID` pattern). A first-party platform agent does not match that shape and
  has no in-repo workflow to bind. This is exactly the "agent is the CI" case and it needs a
  first-class, allowlisted **platform-agent identity** in verifier context.

### 2.4 Orchestration / SDK agents
Claude Agent SDK, OpenAI Agents SDK, LangGraph/Crew/AutoGen-style graphs, MCP-orchestrated runs.

- A run may spawn **subagents** and delegate; the final diff is the product of a delegation tree.
- v0 gap: one flat `agent{}` block cannot express "planner P delegated implementation to child C
  with these tools." The `mcp-orchestrated` `invocationKind` hints at this but carries no data.

---

## 3. Standard-by-standard implications

### 3.1 `AGENTS.md` → attest the operating contract (`agentConfig`)
The single most useful new verifiable fact. `AGENTS.md` is now the portable, cross-harness
instruction set. When a maintainer asks "was this change produced under *my* rules?", the honest
answer is a digest of the instruction files that were in effect — not a guess.

**v1 addition — `agentConfig`:**
- `primaryFile` (e.g. `AGENTS.md`) + `primaryDigest`.
- `files[]`: each `{ path, digest, role }` — role ∈ `{ instructions, tool-rules, repo-policy }`
  so `CLAUDE.md`, `.cursor/rules/**`, `.github/copilot-instructions.md` can be bound without
  privileging one vendor.
- Verifier context gains `verifiedAgentConfig` (digest(s) the verifier recomputed out of band).
  At `policy-grade`/`high-assurance` the default policy requires the declared config digest to
  match the verified one (reusing the stable `missing_evidence` / mismatch vocabulary).

This is a **provenance** claim ("this run executed under instruction set H"), never a claim that
the instructions were followed. The trust anchor remains verified signer/builder + recomputed
subject digests.

### 3.2 MCP → attest tool & server identity with schema digests (`tools`, `mcpServers`)
MCP turned "which tools did the agent call?" into a supply-chain question. OWASP names **tool
poisoning** (a malicious/altered tool description or schema steering the agent) as a top agentic
threat. The deterministic mitigation is to bind **server identity** and the **SHA-256 of each
tool's JSON schema** into the attestation, so a changed tool surface is detectable.

**v1 additions — `mcpServers[]` and `tools[]`:**
- `mcpServers[]`: `{ name, version, transport, serverIdentity, digest }` where `digest` covers
  the server's tool-manifest / schema bundle.
- `tools[]`: `{ name, server, schemaDigest }` — per-tool schema digest.
- Optional at all levels; when present, the default policy requires the declared schema digests
  to be present and (at high-assurance) to match `context.verifiedMcpServers` where the verifier
  can recompute them. No raw tool I/O is ever stored (privacy invariant unchanged).

This deliberately reuses the existing `evidence`/`extension` digest-reference pattern — no new
storage semantics.

### 3.3 Harness-native telemetry → attest *how* the run was captured (`capture`)
If Claude Code (or any OTel-emitting harness) can export its own trace and expose hooks, then
the provenance bundle can be produced **inside** the trusted harness boundary rather than by an
external wrapper. Whether capture was `harness-native`, an external `wrapper` (CLI), or a `ci`
step changes what the evidence-grade is worth and is itself a fact worth binding.

**v1 addition — `capture`:**
- `method` ∈ `{ harness-native, wrapper, ci-step, manual }`.
- `harness` (name/version) when `harness-native`.
- Coupled to `runtime` (OTel trace ids/digest) so a harness-native claim is backed by a trace
  evidence entry — the same binding rule v0 already enforces for `runtime ⇒ trace evidence`.

### 3.4 First-party platform agents → recognized platform identities
GitHub Copilot coding agent, Jules, Codex cloud, etc. authenticate as **platform bots** via the
provider's OIDC, not via a repo workflow. v0's builder shape cannot express this.

**v1 / verifier-context change (no predicate break):**
- Verifier context gains an explicit, allowlisted notion of a **verified platform-agent signer**
  (e.g. `verifiedSigner` carrying the provider's bot identity + issuer). The existing
  `signer_identity_mismatch` / `builder_identity_mismatch` rules already consume
  `context.verifiedSigner` / `context.verifiedBuilderId`; the upgrade is documentation + fixtures
  showing the platform-bot path reaching `policy-grade`, plus keeping `agent.declaredIdentity`
  self-asserted.
- `builder.type` gains a `platform-agent` value so a platform run is distinguishable from a
  repo-owned workflow, without loosening the GitHub-workflow pattern (which still applies when
  `executionType: github-actions`).

### 3.5 Multi-agent / subagent runs → delegation chain (`delegation`)
**v1 addition — `delegation`:**
- `delegationChain[]`: ordered `{ role, agentRef, identity?, digest? }` describing
  parent→child delegation (planner→implementer→reviewer). `agentRef` is a stable reference;
  `digest` optionally binds the sub-run's own attestation.
- Self-asserted like `agent`; only meaningful when the verifier supplies
  `context.verifiedDelegation`. No new trust is created — delegation is descriptive provenance
  unless the parent signer identity is verified.

### 3.6 Trace conventions stay delegated
OpenTelemetry GenAI semantic conventions and OpenInference remain the trace vocabulary.
`runtime.traceFormat` already pins real OTLP media types; v1 keeps referencing traces by id +
digest and **never** copies span payloads into the predicate or public logs. No change beyond
documenting that harness-native OTel export is the preferred capture path.

---

## 4. Gap analysis: April-2026 model vs. September-2026 reality

Legend — **v0**: shipped predicate; **v1**: this upgrade.

| Frontier reality | v0 status | Gap | v1 resolution |
|---|---|---|---|
| `AGENTS.md` is the agent operating contract | Not modeled | The instruction set governing a run is unattested | `agentConfig` (primary + `files[]` digests) + `context.verifiedAgentConfig` |
| MCP tool/server supply chain | Single `mcp-server` evidence *type* only | No server identity, no tool-schema digest → tool poisoning unmeasured | `mcpServers[]`, `tools[]` with schema digests + `context.verifiedMcpServers` |
| Harness-native OTel capture | Assumes CLI/wrapper capture | "How captured" is unmodeled; misses the trusted-boundary case | `capture.method` + `runtime` trace binding |
| First-party platform coding agents | Builder shape assumes repo-owned GH Actions workflow | "Agent is the CI" cannot reach policy-grade | `builder.type: platform-agent` + allowlisted platform-bot `verifiedSigner` + fixtures |
| Multi-agent / subagent delegation | One flat `agent{}` | Delegation tree under-described | `delegation.delegationChain[]` + `context.verifiedDelegation` |
| Agent identity as trust root | Correctly self-asserted | Vocabulary not tied to platform identities | Keep self-asserted; bind to verified platform/workflow signer |
| Roadmap / milestones | April-2026 (VSCode plugin, generic registry) | Stale ordering; misses harness-native + platform-agent paths | Rewritten roadmap (§ of `TASKS.md`) |

**Explicitly still out of scope (unchanged):** code-quality proofs, local-machine-integrity
proofs, a new crypto/PKI/transparency stack, a trace backend, an SBOM/VEX format, an
observability platform, a general artifact registry, and treating `agent.name`/`model.name` as a
trust root.

---

## 5. Design principles carried forward

1. **Preserve v0.** v0 is shipped and its value is interoperability. All v1 work is additive:
   a new predicate URI, new schema/CUE, new fixtures. No v0 field, failure code, or golden
   outcome changes.
2. **Thin composition layer.** v1 binds *references and digests* to `AGENTS.md`, MCP manifests,
   and OTel traces. It stores none of their raw content and reuses Sigstore/DSSE/in-toto/OTel
   unchanged.
3. **Verified data outranks self-assertion.** `agentConfig`, `mcpServers`, `delegation`, and
   `capture` are descriptive until matched against out-of-band verifier context derived from a
   verified envelope / recomputed digests.
4. **Determinism first.** Every new field is schema- and CUE-constrained, policy-gated, and
   covered by golden fixtures with exactly one expected failure code. No check relies on
   judgment.
5. **Stable failure vocabulary.** v1 reuses the existing failure codes; new contract surface is
   the predicate URI and fields, not a churn of public codes.

---

## 6. Source basis

This review is anchored on the project's April-2026 `deep-research-report.md` and the
consolidated public direction of the coding-agent ecosystem through September 2026 (AGENTS.md
steering committee; MCP under the Linux Foundation agentic-AI foundation with an official
registry; OpenTelemetry GenAI semantic conventions; GitHub Copilot coding agent and GitHub
Artifact Attestations custom predicates; Sigstore/cosign/Fulcio/Rekor; OWASP agentic-AI threat
guidance). Where a capability's exact vendor surface is still moving, this document models the
**stable, verifiable core** (a digestable contract + an identity + a schema) rather than any one
vendor's proprietary shape.
