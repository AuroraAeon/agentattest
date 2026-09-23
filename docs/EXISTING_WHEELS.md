# Existing Wheels

`agentattest` composes existing standards and tools. The custom work is limited to an agent-run predicate, deterministic binding rules, verification policy inputs, and developer/CI workflow glue.

| Standard Or Tool | Role In `agentattest` | Reuse | Do Not Implement |
|---|---|---|---|
| in-toto Statement v1 | Outer attestation container | Use `_type`, `subject`, `predicateType`, and `predicate` as the signed claim structure | Do not create a competing statement format or redefine subject semantics |
| DSSE | Signing envelope for statements | Use DSSE-compatible envelopes through cosign, GitHub, or libraries | Do not create a new envelope, canonicalization scheme, or raw signature wrapper |
| Sigstore | Signing and verification ecosystem | Use Sigstore trust roots, bundles, and identity-oriented verification where applicable | Do not operate a custom public-good equivalent in v0 |
| The Update Framework (TUF) | Trust-root distribution and rollback protection | Source the Sigstore community Fulcio root via `sigstore-go/pkg/tuf` when `--trust-root` is omitted (pinned embedded root metadata, hash-verified targets) | Do not implement a custom TUF client, root pinning scheme, or trust-on-first-use fetch |
| go-tuf (`theupdateframework/go-tuf`) | TUF metadata library underneath the Sigstore client | Used transitively through `sigstore-go/pkg/tuf` | Do not hand-roll metadata verification |
| cosign | CLI/library path for signing and verifying attestations | Use `cosign attest`, `cosign verify-attestation`, and policy integrations where appropriate | Do not parse ad hoc signature files when cosign-compatible verification is available |
| Fulcio | OIDC-backed short-lived certificate authority | Reuse certificates issued through Sigstore/GitHub flows | Do not issue certificates or create a project CA |
| Rekor | Transparency log and timestamp evidence | Reuse transparency proofs and signed timestamps through Sigstore bundles | Do not put sensitive raw evidence into Rekor |
| GitHub Artifact Attestations | GitHub-native attestation storage and verification | Use GitHub's attestation APIs and repository association for GitHub-first workflows | Do not create a parallel GitHub attestation store |
| `actions/attest` | GitHub Actions generation path | Use custom predicate mode for `https://agentattest.dev/predicate/v0` | Do not fork the action unless a missing capability blocks v0 |
| `gh attestation` | GitHub CLI verification and download path | Use `gh attestation verify` outputs as upstream verification evidence | Do not rely only on user-controlled predicate fields for signer identity |
| SLSA provenance vocabulary | Build provenance terminology and field inspiration | Reuse concepts such as builder, build type, materials, run details, and verification expectations | Do not force agent-run provenance into SLSA build provenance when the semantics differ |
| OpenTelemetry | Runtime trace transport and general span model | Reference trace IDs, span IDs, exported trace documents, profile labels, and digests | Do not define a trace protocol, collector, or trace storage backend |
| OpenInference | AI-specific trace semantic conventions | Reference agent, LLM, tool, retriever, and chain spans where available | Do not copy full trace payloads into the predicate or public logs |
| SPDX | SBOM sidecar option | Link SPDX documents by URI, media type, and digest | Do not embed SPDX package graphs in the predicate |
| CycloneDX | BOM sidecar option | Link CycloneDX SBOM, ML-BOM, or other BOM documents by URI, media type, and digest | Do not embed CycloneDX object models in the predicate |
| OpenVEX | Vulnerability exploitability sidecar | Link OpenVEX documents by URI, media type, and digest | Do not encode vulnerability status inside the agent provenance predicate |
| SQLite | Local cache and index | Store local run indexes, evidence refs, statement refs, content digests, and last-seen timestamps | Do not store or trust authoritative pass/fail verification decisions |
| OPA/Rego | Policy-as-code | Evaluate repository policy against verified statement and local context | Do not hard-code all repository policy in Go |
| CUE | Schema and semantic constraints | Express stricter predicate constraints and optional policy checks | Do not make CUE the only supported validation path |
| Git | Source tree and patch identity | Use commits, refs, trees, diffs, and normalized digest inputs for binding | Do not replace Git object identity or repository policy |
| `AGENTS.md` | Cross-vendor agent operating contract | Reference and digest the instruction file(s) that governed a run via `agentConfig` | Do not treat the file's contents as a trust root, and do not embed them in the predicate |
| MCP (Model Context Protocol) | Tool / data connectivity plane | Reference MCP server identity and tool-schema digests via `mcpServers` / `tools` | Do not proxy, host, or re-specify MCP; never store raw tool I/O |
| Coding-agent harness telemetry (OTel GenAI conventions + lifecycle hooks) | Run capture | Reference harness-emitted OTel trace ids/digests and record `capture.method` | Do not become a trace collector or re-run the harness |

## Composition Rule

The project owns only these surfaces:

- The custom predicate URI `https://agentattest.dev/predicate/v0` and its `v1` superset.
- JSON Schema and CUE constraints for that predicate.
- Deterministic subject binding rules for patch, tree, PR, and artifact contexts.
- Verifier inputs and failure modes.
- Default Rego/CUE policy examples.
- CLI and GitHub Action ergonomics.

Everything else must remain delegated to existing standards and tools.
