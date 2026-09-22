# Harness capture — reference integrations

`internal/capture` is a thin SDK a coding-agent harness embeds to emit an
agentattest **v1** statement for one run. It records only non-raw metadata and
binds the run to the current repository patch. It never stores raw prompts, raw
tool outputs, or raw trace content.

```go
import (
    "agentattest.dev/agentattest/internal/capture"
    "agentattest.dev/agentattest/internal/gitbind"
)

result, _ := gitbind.Compute(ctx, ".")          // repo URL, base commit, patch digest
stmt, err := capture.Statement(result, capture.RunMetadata{
    AgentName:      "claude-code",
    AgentVersion:   "1.2.3",
    InvocationKind: "local-cli",
    ModelProvider:  "anthropic",
    ModelID:        "claude-sonnet-4",
    AgentConfigPath: "AGENTS.md",               // bound by SHA-256
    TraceFormat:    "otlp-json",
    TraceIDs:       []string{"4bf92f3577b34da6a3ce929d0e0e4736"},
    TraceURI:       "artifact://agentattest/traces/run.otel.json",
    TraceDigest:    "<sha256 of the exported trace document>",
    Now:            time.Now(),
})
// stmt is an in-toto Statement v1 with predicateType .../predicate/v1,
// capture.method = harness-native, a bound trace evidence entry, and the
// AGENTS.md digest. Write it, or sign it via internal/signing.
```

## Local CLI harness (e.g. Claude Code, Codex CLI)

- `InvocationKind: "local-cli"`, `ExecutionType` left as `local` (default).
- Point `TraceURI`/`TraceDigest` at the harness's OTel export. Claude Code and
  Codex can export OTel traces natively; hash the exported document for
  `TraceDigest`.
- Local capture is **evidence-grade** — it cannot be policy-grade without a
  verified non-local signer.

## Platform coding agent (e.g. GitHub Copilot coding agent, Codex cloud)

- `InvocationKind: "platform-agent"`, `ExecutionType: "platform-agent"`.
- The run executes in a provider-controlled environment; to reach **policy-grade**,
  sign the statement with the platform's keyless identity and verify via
  `agentattest verify bundle` (see `action/README.md`).
- Record the MCP surface used: `MCPServers` (server identity + tool-manifest
  digest) and `Tools` (per-tool schema digest) — the deterministic mitigation for
  tool/schema poisoning.

## Guarantees

- **Fail-closed:** harness-native capture requires a bound trace (IDs + URI +
  digest); a missing or malformed digest is an error, not a silent omission.
- **Privacy:** no raw prompt/tool/trace is captured; only digests, identities,
  and references. `rawPrompt.stored` / `rawToolOutputs.stored` stay `false`.
- **Determinism:** the operating contract and trace are bound by SHA-256, so a
  verifier can recompute and compare them out of band.
