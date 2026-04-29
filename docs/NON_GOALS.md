# Non-Goals

These are hard boundaries for v0. They prevent `agentattest` from becoming another cryptography stack, trace platform, SBOM format, or broad AI safety product.

## New Signature System

`agentattest` will not create signature primitives, key formats, PKI, timestamping, certificate issuance, or transparency logs.

Why: DSSE, Sigstore, cosign, Fulcio, Rekor, and GitHub Artifact Attestations already cover this plane. Reimplementing it would increase risk and reduce interoperability.

## New Attestation Container

`agentattest` will not define a replacement for in-toto Statement v1.

Why: The useful extension point is the predicate. The outer claim structure must remain interoperable with existing attestation tools.

## New Trace Protocol

`agentattest` will not define a runtime trace wire format or span vocabulary.

Why: OpenTelemetry and OpenInference already define trace structure and AI-specific semantic conventions. The predicate should reference trace evidence, not replace it.

## New SBOM Or VEX Format

`agentattest` will not define package inventory, dependency inventory, vulnerability status, or exploitability schemas.

Why: SPDX, CycloneDX, and OpenVEX already solve those problems. Agent provenance can link them as sidecars.

## New Observability Platform

`agentattest` will not build a Langfuse, Phoenix, MLflow, or trace-search replacement.

Why: Observability products store, search, and visualize runtime traces. `agentattest` verifies signed binding claims and policy decisions.

## Code Quality Proof

`agentattest` will not claim that AI-generated code is correct, secure, maintainable, or production-ready.

Why: Provenance can prove a claim was made about a subject under a policy. It cannot prove semantic code quality.

## Local Machine Integrity Proof

`agentattest` will not claim that a developer laptop, local shell, IDE, or self-hosted runner was uncompromised.

Why: Local capture can provide useful evidence, but without stronger runtime isolation and identity controls it cannot establish a high-trust execution environment.

## Raw Prompt Archival By Default

`agentattest` will not store raw prompts, raw tool outputs, raw retrieved documents, or raw model responses by default.

Why: These often contain secrets, customer data, source code, personal data, credentials, and internal business context.

## Public Sensitive Logging

`agentattest` will not write sensitive raw evidence into public transparency logs or public PR comments.

Why: Public logs are durable and broadly replicated. They are appropriate for signatures, digests, identities, and minimal summaries, not private evidence blobs.

## Agent Identity As A Trust Root

`agentattest` will not treat the agent name or model name in the predicate as independently trustworthy.

Why: These fields are useful metadata. Trust comes from verified signer identity, builder identity, workflow policy, subject binding, and evidence constraints.

## Universal Policy Oracle

`agentattest` will not decide whether every repository should accept an AI-authored change.

Why: Repository owners define policy. The project provides deterministic inputs, default policies, and verification mechanics.

## Secret Management System

`agentattest` will not become a vault, DLP platform, or secret scanner of record.

Why: It should integrate with existing secret scanning and redaction controls while enforcing safe defaults for its own data model.

## General Artifact Registry

`agentattest` will not become a general artifact registry or long-term evidence object store in v0.

Why: GitHub Attestations, OCI registries, object stores, and existing artifact systems already cover distribution and retention.

## VSCode Extension In v0

`agentattest` will not include a VSCode extension in v0.

Why: The first release should stabilize the CLI, schema, verifier, policy, and GitHub Action surfaces before adding IDE UX.
