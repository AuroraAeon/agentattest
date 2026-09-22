// Package capture is a thin SDK a coding-agent harness embeds to emit an
// agentattest v1 statement describing one of its runs. It records only non-raw
// metadata — agent/model identity, the operating contract (AGENTS.md) by digest,
// OpenTelemetry trace references, and optional MCP server/tool identity — and
// binds them to the current repository patch. It never captures raw prompts,
// raw tool outputs, or raw trace content.
//
// A harness calls Statement with the git binding it computed and the run
// metadata it already has, and writes the returned in-toto Statement v1 (or
// signs it via internal/signing). Example integrations for a local CLI harness
// and a platform agent live in docs/examples/capture.md.
package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
)

// RunMetadata is the harness-supplied, non-raw description of one run.
type RunMetadata struct {
	AgentName      string
	AgentVersion   string
	InvocationKind string // local-cli | ci-step | api | mcp-orchestrated | platform-agent (default local-cli)
	ExecutionType  string // local | github-actions | other-ci | platform-agent | ... (default local)
	ModelProvider  string
	ModelID        string

	// AgentConfigPath is the operating contract file (e.g. AGENTS.md) to bind by
	// SHA-256. Optional.
	AgentConfigPath string

	// Trace references (OTel). Harness-native capture requires a bound trace:
	// TraceIDs plus the exported trace document's URI and digest.
	TraceFormat string // otlp-json | opentelemetry-json | openinference-otel-json | otlp-protobuf (default otlp-json)
	TraceIDs    []string
	RootSpanID  string
	TraceURI    string
	TraceDigest string // hex sha256 of the exported trace document

	// Optional MCP surface (identity + schema digests only).
	MCPServers []MCPServer
	Tools      []Tool

	Now time.Time
}

// MCPServer records an MCP server identity and the digest of its tool manifest.
type MCPServer struct {
	Name           string
	Version        string
	Transport      string // stdio | streamable-http | sse
	ServerIdentity string
	Digest         string // hex sha256 of the tool-manifest / schema bundle
}

// Tool records a per-tool schema digest.
type Tool struct {
	Name         string
	Server       string
	SchemaDigest string
}

var hexSha256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Statement builds a harness-native v1 in-toto Statement v1 binding the run to
// the repository patch. It fails closed when a harness-native run has no bound
// trace, when the operating-contract file cannot be read, or when a trace
// digest is malformed.
func Statement(result gitbind.Result, meta RunMetadata) (map[string]any, error) {
	if len(meta.TraceIDs) == 0 || meta.TraceURI == "" || meta.TraceDigest == "" {
		return nil, errors.New("capture: harness-native capture requires trace IDs, a trace URI, and a trace digest")
	}
	if !hexSha256.MatchString(meta.TraceDigest) {
		return nil, errors.New("capture: trace digest must be a lowercase hex sha256")
	}

	opts := predicate.CreateOptions{
		AgentName:      meta.AgentName,
		AgentVersion:   meta.AgentVersion,
		CaptureMethod:  "harness-native",
		CaptureHarness: meta.AgentName,
		ModelProvider:  meta.ModelProvider,
		ModelID:        meta.ModelID,
		Now:            meta.Now,
	}

	if meta.AgentConfigPath != "" {
		data, err := os.ReadFile(meta.AgentConfigPath)
		if err != nil {
			return nil, fmt.Errorf("capture: read operating contract %q: %w", meta.AgentConfigPath, err)
		}
		sum := sha256.Sum256(data)
		opts.AgentConfigPath = filepath.Base(meta.AgentConfigPath)
		opts.AgentConfigSHA256 = hex.EncodeToString(sum[:])
	}

	pred := predicate.CreateV1(result, opts)

	agent := pred["agent"].(map[string]any)
	if meta.InvocationKind != "" {
		agent["invocationKind"] = meta.InvocationKind
	}
	env := pred["environment"].(map[string]any)
	if meta.ExecutionType != "" {
		env["executionType"] = meta.ExecutionType
	}

	traceFormat := meta.TraceFormat
	if traceFormat == "" {
		traceFormat = "otlp-json"
	}
	runtime := map[string]any{
		"traceFormat": traceFormat,
		"traceIds":    meta.TraceIDs,
	}
	if meta.RootSpanID != "" {
		runtime["rootSpanId"] = meta.RootSpanID
	}
	pred["runtime"] = runtime

	// The trace document is the primary evidence for a harness-native run.
	evidence := []map[string]any{
		{
			"type":       "trace",
			"uri":        meta.TraceURI,
			"digest":     map[string]any{"sha256": meta.TraceDigest},
			"mediaType":  traceMediaType(traceFormat),
			"storage":    "artifact-store",
			"visibility": "team",
		},
	}
	pred["evidence"] = evidence

	if len(meta.MCPServers) > 0 {
		servers := make([]map[string]any, 0, len(meta.MCPServers))
		for _, s := range meta.MCPServers {
			entry := map[string]any{
				"name":      s.Name,
				"transport": s.Transport,
				"digest":    map[string]any{"sha256": s.Digest},
			}
			if s.Version != "" {
				entry["version"] = s.Version
			}
			if s.ServerIdentity != "" {
				entry["serverIdentity"] = s.ServerIdentity
			}
			servers = append(servers, entry)
		}
		pred["mcpServers"] = servers
	}
	if len(meta.Tools) > 0 {
		tools := make([]map[string]any, 0, len(meta.Tools))
		for _, tl := range meta.Tools {
			entry := map[string]any{
				"name":         tl.Name,
				"schemaDigest": map[string]any{"sha256": tl.SchemaDigest},
			}
			if tl.Server != "" {
				entry["server"] = tl.Server
			}
			tools = append(tools, entry)
		}
		pred["tools"] = tools
	}

	return predicate.StatementForV1(result, pred), nil
}

func traceMediaType(format string) string {
	switch format {
	case "otlp-protobuf":
		return "application/x-protobuf"
	default:
		return "application/json"
	}
}
