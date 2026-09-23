// v1 predicate cross-field constraints.
//
// LOAD CONTRACT: this file is loaded STANDALONE by file path (see
// internal/verify.validateCUE, which calls load.Instances with this exact path).
// Do NOT run `cue vet ./schemas/` as a whole package: agent-provenance-v0.cue and
// this file both declare `package schemas` and would collide on shared defs.
// Validate a single file: `cue vet schemas/agent-provenance-v1.cue <fixture>`.
package schemas

#HexSha256: =~"^[a-f0-9]{64}$"
#GitCommit: =~"^([a-f0-9]{40}|[a-f0-9]{64})$"
#TraceID: =~"^[a-f0-9]{32}$"
#SpanID: =~"^[a-f0-9]{16}$"
#SafeURI: string & =~"^(?:(?:https?://[^\\s/@:]+(?::[0-9]+)?(?:/[^\\s]*)?)|(?:artifact|s3|oci)://[^\\s]+|(?:spdx|cyclonedx):[^\\s]+)$"
#RepoHTTPURI: string & =~"^https?://[^\\s/@:]+(?::[0-9]+)?(?:/[^\\s]*)?$"
#RFC3339UTC: string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?Z$"
#GitHubWorkflowID: string & =~"^https://github\\.com/[^/]+/[^/]+/\\.github/workflows/[^@]+@.+$"
#RelPath: string & =~"^[A-Za-z0-9._/*-]{1,256}$"

#Digest: close({
	sha256: #HexSha256
})

#Visibility: "public" | "team" | "secret" | "encrypted"
#RawVisibility: "none" | "team" | "secret" | "encrypted"

#Repo: close({
	url:         #RepoHTTPURI
	baseCommit: #GitCommit
	headCommit?: #GitCommit
	branch?:    string & =~"^[A-Za-z0-9._/-]{1,255}$"
	pullRequest?: close({
		number: int & >=1
		url?:   #RepoHTTPURI
	})
})

#Agent: close({
	name:           string & =~"^.{1,128}$"
	version:        string & =~"^.{1,128}$"
	invocationKind: "local-cli" | "ci-step" | "api" | "mcp-orchestrated" | "platform-agent"
	declaredIdentity?: string & =~"^[A-Za-z0-9._:/-]{1,256}$"
})

#Model: close({
	provider:       string & =~"^.{1,128}$"
	modelId:        string & =~"^.{1,256}$"
	modelRevision?: string & =~"^.{1,128}$"
	fingerprint?:   string & =~"^.{1,256}$"
})

#Builder: close({
	id:    string & =~"^.{1,512}$"
	type:  "github-actions-workflow" | "generic-ci-workflow" | "platform-agent" | "local-process" | "isolated-runner" | "witnessed-runner"
	workflowRef?: string & =~"^.{1,512}$"
	runnerClass?: "github-hosted" | "self-hosted" | "isolated" | "platform-managed" | "unknown"
})

#Witness: close({
	id:     string & =~"^.{1,512}$"
	type:   "transparency-log" | "timestamp-authority"
	digest: #Digest
	uri?:   #SafeURI
})

#Environment: close({
	executionType: "local" | "github-actions" | "other-ci" | "platform-agent" | "isolated-runner" | "witnessed-runner"
	builder?:     #Builder
	witnesses?:   [...#Witness]

	if executionType == "github-actions" {
		builder: #Builder & {
			id:          #GitHubWorkflowID
			type:        "github-actions-workflow"
			workflowRef: string & =~"^.{1,512}$"
		}
	}
})

#Capture: close({
	method:   "harness-native" | "wrapper" | "ci-step" | "manual"
	harness?: string & =~"^.{1,128}$"

	if method == "harness-native" {
		harness: string & =~"^.{1,128}$"
	}
})

#AgentConfig: close({
	primaryFile:    #RelPath
	primaryDigest:  #Digest
	files?:         [...#ConfigFile]
})

#ConfigFile: close({
	path:   #RelPath
	digest: #Digest
	role:   "instructions" | "tool-rules" | "repo-policy"
})

#McpServer: close({
	name:           string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$"
	version?:       string & =~"^.{1,64}$"
	transport:      "stdio" | "streamable-http" | "sse"
	serverIdentity?: string & =~"^.{1,512}$"
	digest:         #Digest
})

#Tool: close({
	name:         string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$"
	server?:      string & =~"^.{1,128}$"
	schemaDigest: #Digest
})

#Delegation: close({
	delegationChain: [#DelegationStep, ...#DelegationStep]
})

#DelegationStep: close({
	role:     string & =~"^.{1,64}$"
	agentRef: string & =~"^.{1,256}$"
	identity?: string & =~"^[A-Za-z0-9._:/-]{1,256}$"
	digest?:  #Digest
})

#Runtime: close({
	traceFormat: "opentelemetry-json" | "openinference-otel-json" | "otlp-json" | "otlp-protobuf"
	traceIds:    [#TraceID, ...#TraceID]
	rootSpanId?: #SpanID
})

#Changes: close({
	filesChanged:   int & >=0
	insertions:     int & >=0
	deletions:      int & >=0
	generatedFiles?: int & >=0
})

#HumanApproval: close({
	state: "none" | "not-required" | "requested" | "approved-after-generation" | "approved-before-merge" | "rejected"
	reviewerRefs?: [...string & =~"^[a-z]+:[a-z]+:[A-Za-z0-9._-]{1,200}$"]
	approvalSystem?: string & =~"^.{1,128}$"
})

#RawEvidence: close({
	stored:        bool
	visibility:    #RawVisibility
	encryptedRef?: #SafeURI
	retention?:     string & =~"^.{1,128}$"

	if stored == false {
		visibility: "none"
	}
	if stored == true {
		visibility: "team" | "secret" | "encrypted"
	}
	if visibility == "encrypted" {
		encryptedRef: #SafeURI
	}
})

#Privacy: close({
	redactionPolicy: string & =~"^.{1,128}$"
	rawPrompt:       #RawEvidence
	rawToolOutputs:  #RawEvidence
	publicTransparencyLog: close({
		allowed:            bool
		includesRawContent: false
	})
})

#Evidence: close({
	type:       "trace" | "tool-summary" | "test-result" | "local-log-root" | "builder-record" | "approval" | "mcp-server" | "agent-config" | "raw-prompt-ref" | "raw-tool-output-ref"
	uri:        #SafeURI
	digest:     #Digest
	mediaType?: string & =~"^.{1,256}$"
	storage:    "local-only" | "repo" | "artifact-store" | "transparency-log" | "encrypted-blob"
	visibility: #Visibility

	if type == "raw-prompt-ref" {
		visibility: "team" | "secret" | "encrypted"
	}
	if type == "raw-tool-output-ref" {
		visibility: "team" | "secret" | "encrypted"
	}
})

#Sidecar: close({
	type:       "spdx" | "cyclonedx" | "openvex" | "other"
	uri:        #SafeURI
	digest:     #Digest
	mediaType:  string & =~"^.{1,256}$"
	visibility: #Visibility
})

#Material: close({
	type:   "git-repo" | "issue" | "pull-request" | "artifact" | "dependency-snapshot" | "other"
	uri:    #SafeURI
	digest: #Digest
})

#Extension: close({
	uri:        #SafeURI
	digest:     #Digest
	mediaType:  string & =~"^.{1,256}$"
	visibility: #Visibility
	retention?: string & =~"^.{1,128}$"
})

#Timestamps: close({
	startedAt:  #RFC3339UTC
	finishedAt: #RFC3339UTC & >=startedAt
})

#AgentProvenanceBase: {
	predicateVersion: string & =~"^v[0-9]+$"
	runId:            string & =~"^[A-Za-z0-9._:-]{8,128}$"
	verificationLevel: "evidence-grade" | "policy-grade" | "high-assurance"

	repo:          #Repo
	agent:         #Agent
	model?:        #Model
	environment:   #Environment
	capture?:      #Capture
	agentConfig?:  #AgentConfig
	mcpServers?:   [...#McpServer]
	tools?:        [...#Tool]
	delegation?:   #Delegation
	runtime?:      #Runtime
	changes?:      #Changes
	humanApproval: #HumanApproval
	privacy:       #Privacy
	timestamps:    #Timestamps
	evidence:      [#Evidence, ...#Evidence]
	sidecars?:     [...#Sidecar]
	materials?:    [...#Material]
	extensions?: {
		[=~"^https?://"]: #Extension
	}

	if runtime != _|_ {
		_traceEvidence: [for e in evidence if e.type == "trace" {e}]
		_traceEvidence: [_, ...]
	}

	if capture != _|_ {
		if capture.method == "harness-native" {
			runtime: #Runtime
			_nativeTrace: [for e in evidence if e.type == "trace" {e}]
			_nativeTrace: [_, ...]
		}
	}
}

#AgentProvenanceV1: close(#AgentProvenanceBase)
