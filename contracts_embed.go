// Package agentattest embeds the contract pack — predicate schemas, CUE
// definitions, the default verification policy, and the verifier-context
// schema — so the exported wrapper in pkg/agentattest can verify without an
// agentattest checkout on disk. The files are embedded byte-for-byte.
package agentattest

import "embed"

// Contracts holds the contract pack at the paths contracts.Locate expects.
//
//go:embed schemas/agent-provenance-v0.schema.json schemas/agent-provenance-v0.cue
//go:embed schemas/agent-provenance-v1.schema.json schemas/agent-provenance-v1.cue
//go:embed policies/default.rego tests/golden/context.schema.json
var Contracts embed.FS
