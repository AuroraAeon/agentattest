# Threat Model

The v0 threat model assumes the attacker may control some local files, tool outputs, agent metadata, or workflow inputs. The verifier must distinguish user-controlled predicate fields from verified envelope, signer, builder, timestamp, and local recomputation inputs.

| Threat | Mitigation | Residual Risk | v0 Handling |
|---|---|---|---|
| Tampered local log | Hash evidence files and store only digest-addressed references in the predicate | If the local machine is compromised before signing, attacker can forge local evidence | Evidence-grade only unless the evidence is produced in a verified CI or witnessed environment |
| Replayed attestation | Bind repo URL, base commit, and exact subject sets by default; optionally check PR number, run ID, signer, workflow, and freshness via explicit verifier context | A verifier with incomplete context may accept an old statement | Default policy checks repo URL, base commit, exact subjects, and any provided PR/run/freshness context; configured freshness fails closed when `now` is absent |
| Forged agent identity | Treat `agent` and `model` as self-asserted metadata; require `agent.declaredIdentity` to match `context.verifiedSigner` when declared | A trusted workflow can still lie about agent metadata if compromised | v0 displays agent identity as declared unless backed by verified signer context |
| Forged builder identity | Verify DSSE/Sigstore/GitHub certificate, OIDC, workflow, issuer, and repository identity outside the predicate | Self-hosted or compromised CI runners can produce misleading but signed claims | Policy-grade requires verified non-local builder/workflow/issuer context; high-assurance also requires runner allowlist and witness verification context |
| Post-attestation code modification | Recompute patch, tree, PR, or artifact digest immediately before verification or merge gate | Race conditions remain if code changes after verification and before merge | Subject mismatch fails verification; merge gates should verify at the final merge base |
| Prompt/tool-output privacy leakage | Do not store raw prompts/tool outputs by default; classify visibility; redact before persistence; prohibit public raw evidence and transparency-log content-shaped storage | Structural checks do not scan referenced content and redaction can miss secrets or personal data | JSON Schema + CUE forbid public raw evidence, inline extension blobs, credentialed URLs, and `data:` payloads; Rego forbids content-shaped evidence and non-public evidence on transparency logs and public extensions |
| MCP/tool poisoning | Record tool/server identity, schema digest, tool evidence digest, and tool summaries; verifier allowlists approved server manifests via `context.verifiedMcpServers` | A poisoned tool can still influence generated code before detection | v0 treats MCP/tool evidence as context, not trust; there is no per-tool risk model and no tool-triggered approval gate in v0/v1 |

## v1 Threat Additions

v1 binds the harness plane, which introduces a few new threats. Each is handled by treating the new fields as self-asserted and requiring out-of-band verified context before they carry weight.

| Threat | Mitigation | Residual Risk |
|---|---|---|
| Operating-contract substitution (a different `AGENTS.md` was actually in effect) | Bind `agentConfig.primaryDigest`; verifier recomputes the digest and rejects a mismatch via `context.verifiedAgentConfig` | A compromised harness can sign a truthful digest of a poisoned instructions file; this proves *which* contract ran, not that it was safe |
| MCP server / tool-schema swap | Bind `mcpServers[].digest`; verifier allowlists approved manifests via `context.verifiedMcpServers`, and at policy-grade or above a declared `mcpServers` list without a verified allowlist fails closed. `tools[].schemaDigest` binds the tool surface by digest in the predicate but is **not** policy-checked (no `verifiedTools` context exists) | An allowlisted server can still serve a malicious tool at runtime; a schema digest detects surface changes, not behavior |
| Delegation / subagent identity confusion | `delegation.delegationChain` is self-asserted; every step's `agentRef` must be in `context.verifiedDelegation` when the verifier supplies one, and at policy-grade or above a declared chain without a verified allowlist fails closed | A verified parent can delegate to an unverified child; the chain documents structure, it does not attest the child |
| Harness-native capture spoofing (claiming `harness-native` without a real trace) | `capture.method == "harness-native"` structurally requires a `runtime` trace and a `trace` evidence entry (CUE); high-assurance additionally requires bound trace evidence and rejects `capture.method: manual` | A forged trace can still be produced; only a verified signer plus a recomputed digest raise assurance |
| Platform-agent identity spoofing | `builder.type: platform-agent` has no dedicated rule; it flows through the same generic verified-identity gates as every other builder (verified signer / builder ID / workflow ref / issuer at policy-grade and above) | A compromised platform account can still produce misleading but signed claims |

## Trust Boundaries

- Verified envelope data is stronger than predicate data.
- Current local or CI recomputation of subject digests is stronger than stored cache data.
- GitHub/Sigstore certificate identity is stronger than `agent.name`.
- Human approval state in the predicate is weaker than platform-enforced review state.
- OpenTelemetry/OpenInference traces are evidence, not proof of correctness.
- Sidecar SBOM/VEX documents are evidence, not replacements for subject binding.

## v0 Security Defaults

- Fail closed on schema, predicate type, subject digest, repo URL, base commit, privacy, required-level, and level-escalation failures.
- Require statement subjects and verifier context subjects to be equal sets.
- Do not allow `local-only` evidence for policy-grade or high-assurance.
- Require explicit verifier context for signer, builder, workflow, issuer, high-assurance runner, witness, and approval digest checks.
- Require explicit policy to trust self-hosted runners.
- Require digest-addressed evidence and extension references.
- Keep raw prompt and raw tool output capture opt-in.
- Keep public-log payloads structurally minimal and privacy-safe.
