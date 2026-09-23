# AGENTS.md

This repository is `agentattest`: a thin interoperability layer for AI coding-agent provenance and attestation.

Use this file as a map, not a manual. Follow the linked contract documents before changing code, schemas, or policy.

## Mission And Boundaries

- Product mission and original research: `deep-research-report.md`.
- Hard non-goals: `docs/NON_GOALS.md`.
- Existing standards and tools to reuse: `docs/EXISTING_WHEELS.md`.
- Implementation roadmap and acceptance criteria: `TASKS.md`.

## Architecture Map

- Layering, module boundaries, and dependency directions: `ARCHITECTURE.md`.
- Data model and predicate contract: `docs/DATA_MODEL.md`.
- Verification semantics and failure modes: `docs/VERIFICATION_MODEL.md`.
- Security threats and v0 mitigations: `docs/THREAT_MODEL.md`.
- Privacy defaults and visibility classes: `docs/PRIVACY_MODEL.md`.
- Security review evidence for the release checklist: `docs/SECURITY_REVIEW.md`.

## Schema And Policy Map

- JSON Schema for the custom predicate only: `schemas/agent-provenance-v0.schema.json`.
- CUE constraints for the custom predicate: `schemas/agent-provenance-v0.cue`.
- v1 superset (frontier-harness bindings): `schemas/agent-provenance-v1.schema.json` + `schemas/agent-provenance-v1.cue`.
- Default OPA/Rego policy: `policies/default.rego`.
- Golden fixture structure: `tests/golden/README.md`.

## Strict Rules

- Do not invent cryptography, key management, PKI, signing, timestamping, or transparency logs.
- Do not invent a new attestation container. Use in-toto Statement v1 and DSSE-compatible signing flows.
- Do not invent a trace protocol. Store references to OpenTelemetry/OpenInference traces only.
- Do not invent an SBOM or VEX format. Use SPDX, CycloneDX, and OpenVEX only as sidecars.
- Do not store raw prompts or raw tool outputs by default.
- Do not write raw prompts, tool outputs, secrets, customer code excerpts, or personal data into public transparency logs.
- Do not claim agentattest proves code quality, model correctness, or that a local machine was uncompromised.
- Treat agent identity in the predicate as self-asserted unless backed by signer, builder, or workflow identity.
- The default verifier MUST emit `signer_identity_mismatch` whenever `predicate.agent.declaredIdentity` is non-empty and does not match `context.verifiedSigner`.
- Treat builder identity as verified only when it comes from the verified envelope, certificate, CI identity, or trusted policy input.
- Tests are required for every schema change.
- Golden fixtures and policy tests must be updated with every predicate field addition, removal, or semantic change.
- Schema objects must forbid unplanned fields. Add explicit extension points instead of accepting arbitrary data.
- Verification must fail closed on unsupported predicate versions, missing required evidence, and privacy violations.

## Versioning Rules

- Predicate type for v0 is exactly `https://agentattest.dev/predicate/v0`.
- Predicate type for v1 is exactly `https://agentattest.dev/predicate/v1`; `predicateVersion` is exactly `v1`. The verifier accepts both v0 and v1 and routes schema/CUE by type.
- Breaking predicate changes require a new predicate URI and new golden fixtures.
- Non-breaking additions must be optional, documented, schema-constrained, and covered by tests.

## Default Implementation Bias

- Core CLI and verifier: Go.
- Predicate contracts: JSON Schema and CUE.
- Policy checks: Rego or CUE, with deterministic inputs.
- Local cache and indexing: SQLite.
- GitHub integration: Go binary or composite action.
- VSCode extension: out of scope for v0.

## Review Checklist For Agents

- Read `docs/NON_GOALS.md` before proposing scope expansion.
- Read `ARCHITECTURE.md` before adding packages or dependencies.
- Read `docs/DATA_MODEL.md` before touching predicate fields.
- Read `docs/PRIVACY_MODEL.md` before adding captured data.
- Read `docs/VERIFICATION_MODEL.md` before changing verifier behavior.
- Add or update golden fixtures for schema or policy changes.
