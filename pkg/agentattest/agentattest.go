// Package agentattest is a thin exported wrapper over agentattest's internal
// packages so other Go modules can create and verify agentattest statements as
// a library. It adds no verification or cryptographic logic: every call
// delegates to the internal package that owns the behavior, and the contract
// pack is the embedded, byte-identical copy of schemas/, policies/, and the
// verifier-context schema.
package agentattest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	root "agentattest.dev/agentattest"
	"agentattest.dev/agentattest/internal/contracts"
	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
	"agentattest.dev/agentattest/internal/signing"
	"agentattest.dev/agentattest/internal/verify"
)

// BaseCommit is the upstream commit this wrapper is built on.
const BaseCommit = "fabfab962b68a06828ea8608d23da36e4b3ddcce"

// Predicate types (unchanged upstream contracts).
const (
	PredicateTypeV0 = "https://agentattest.dev/predicate/v0"
	PredicateTypeV1 = "https://agentattest.dev/predicate/v1"
)

// FailureCodes lists upstream's stable failure codes in priority order.
var FailureCodes = []string{
	"unsupported_statement_type", "predicate_type_mismatch", "unsupported_predicate_version",
	"schema_invalid", "signature_invalid", "transparency_verification_failed",
	"subject_digest_mismatch", "repo_url_mismatch", "base_commit_mismatch",
	"signer_identity_mismatch", "builder_identity_mismatch", "replay_detected",
	"privacy_violation", "level_escalation", "level_below_required",
	"missing_evidence", "stale_attestation", "policy_eval_error",
}

// Result is the verifier outcome: {valid, level, failureCodes}.
type Result struct {
	Valid        bool     `json:"valid"`
	Level        string   `json:"level,omitempty"`
	FailureCodes []string `json:"failureCodes"`
}

// OrderFailureCodes applies upstream's deterministic presentation order.
func OrderFailureCodes(codes []string) []string { return verify.OrderFailureCodes(codes) }

var (
	packOnce  sync.Once
	packFiles contracts.Files
	packErr   error
)

// contractPack materializes the embedded contract pack once per process in a
// private temporary directory, because the upstream verifier reads contracts
// by path. Extracted bytes are checked against the embedded bytes.
func contractPack() (contracts.Files, error) {
	packOnce.Do(func() {
		dir, err := os.MkdirTemp("", "agentattest-contracts-")
		if err != nil {
			packErr = err
			return
		}
		err = fs.WalkDir(root.Contracts, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := fs.ReadFile(root.Contracts, p)
			if err != nil {
				return err
			}
			dst := filepath.Join(dir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(dst, b, 0o400); err != nil {
				return err
			}
			got, err := os.ReadFile(dst)
			if err != nil || !bytes.Equal(got, b) {
				return fmt.Errorf("contract %s did not extract byte-identically", p)
			}
			return nil
		})
		if err != nil {
			packErr = err
			return
		}
		packFiles, packErr = contracts.Locate(dir)
	})
	return packFiles, packErr
}

// Verify runs upstream's five-phase verifier (routing, JSON Schema, CUE, Rego,
// dedupe/order) against the embedded contract pack. Any internal error fails
// closed with upstream's own codes.
func Verify(ctx context.Context, statementJSON, contextJSON []byte) Result {
	files, err := contractPack()
	if err != nil {
		return Result{FailureCodes: []string{"schema_invalid"}}
	}
	r := verify.Predicate(ctx, statementJSON, contextJSON, verify.Options{Contracts: files})
	return Result{Valid: r.Valid, Level: r.Level, FailureCodes: r.FailureCodes}
}

// ContractDigests returns the SHA-256 of every embedded contract file, keyed by
// its repository-relative path. Consumers compare it with upstream at the
// pinned commit to detect drift.
func ContractDigests() map[string]string {
	out := map[string]string{}
	_ = fs.WalkDir(root.Contracts, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := fs.ReadFile(root.Contracts, p)
		sum := sha256.Sum256(b)
		out[p] = hex.EncodeToString(sum[:])
		return nil
	})
	return out
}

// ChangedFile is one file in a git binding.
type ChangedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Status string `json:"status"` // present | deleted
}

// GitBinding binds a statement to a repository change.
type GitBinding struct {
	RepoURL      string        `json:"repoUrl"`
	Branch       string        `json:"branch,omitempty"`
	BaseCommit   string        `json:"baseCommit"`
	HeadCommit   string        `json:"headCommit"`
	PatchSHA256  string        `json:"patchSha256"`
	ChangedFiles []ChangedFile `json:"changedFiles"`
}

func (b GitBinding) internal() gitbind.Result {
	r := gitbind.Result{
		RepoURL: b.RepoURL, Branch: b.Branch, BaseCommit: b.BaseCommit, HeadCommit: b.HeadCommit,
		Patch: gitbind.Digest{Algorithm: "sha256", Digest: b.PatchSHA256},
	}
	for _, f := range b.ChangedFiles {
		r.ChangedFiles = append(r.ChangedFiles, gitbind.FileDigest{Path: f.Path, SHA256: f.SHA256, Status: f.Status})
	}
	return r
}

// BindWorkingTree is upstream's local-capture binding: the uncommitted working
// tree diff against the upstream merge base (gitbind.Compute).
func BindWorkingTree(ctx context.Context, repoDir string) (GitBinding, error) {
	r, err := gitbind.Compute(ctx, repoDir)
	if err != nil {
		return GitBinding{}, err
	}
	b := GitBinding{RepoURL: r.RepoURL, Branch: r.Branch, BaseCommit: r.BaseCommit, HeadCommit: r.HeadCommit, PatchSHA256: r.Patch.Digest}
	for _, f := range r.ChangedFiles {
		b.ChangedFiles = append(b.ChangedFiles, ChangedFile{Path: f.Path, SHA256: f.SHA256, Status: f.Status})
	}
	return b, nil
}

// BindRange binds a committed change for CI: the patch is `git diff --binary
// base head` and changed files are hashed at head. repoURL is normalized with
// upstream's NormalizeRepoURL.
func BindRange(ctx context.Context, repoDir, repoURL, base, head string) (GitBinding, error) {
	url, err := gitbind.NormalizeRepoURL(repoURL)
	if err != nil {
		return GitBinding{}, err
	}
	patch, err := gitOut(ctx, repoDir, "diff", "--binary", "--no-color", base, head)
	if err != nil {
		return GitBinding{}, err
	}
	sum := sha256.Sum256(patch)
	names, err := gitOut(ctx, repoDir, "diff", "--name-only", "-z", base, head)
	if err != nil {
		return GitBinding{}, err
	}
	b := GitBinding{RepoURL: url, BaseCommit: base, HeadCommit: head, PatchSHA256: hex.EncodeToString(sum[:])}
	for _, n := range bytes.Split(names, []byte{0}) {
		if len(n) == 0 {
			continue
		}
		p := path.Clean(string(n))
		content, err := gitOut(ctx, repoDir, "show", head+":"+p)
		if err != nil {
			b.ChangedFiles = append(b.ChangedFiles, ChangedFile{Path: p, Status: "deleted"})
			continue
		}
		fileSum := sha256.Sum256(content)
		b.ChangedFiles = append(b.ChangedFiles, ChangedFile{Path: p, SHA256: hex.EncodeToString(fileSum[:]), Status: "present"})
	}
	sort.Slice(b.ChangedFiles, func(i, j int) bool { return b.ChangedFiles[i].Path < b.ChangedFiles[j].Path })
	return b, nil
}

func gitOut(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out.Bytes(), nil
}

// StatementOptions describe a run. All fields are self-asserted by the
// emitter; verifiers must not treat them as verified identity.
type StatementOptions struct {
	AgentName      string
	AgentVersion   string
	CaptureMethod  string // wrapper | ci-step | manual
	InvocationKind string // local-cli | ci-step | api | mcp-orchestrated | platform-agent
	ExecutionType  string // local | github-actions | other-ci | platform-agent
	ModelProvider  string
	ModelID        string
	Now            time.Time

	// VerificationLevel claimed by the statement (default evidence-grade).
	// The verifier only grants it when the context supplies matching verified
	// identity; claiming more than is verified fails with level_escalation.
	VerificationLevel string
	// Builder describes the CI workflow (policy-grade and above). Rego compares
	// it with verifiedBuilderId / verifiedWorkflowRef from the signature.
	Builder *Builder
	// EvidenceStorage overrides the default "local-only" storage of the
	// generated tool-summary reference (policy-grade forbids local-only).
	EvidenceStorage string
}

// Builder is the predicate's environment.builder object.
type Builder struct {
	ID          string
	Type        string // e.g. github-actions-workflow
	WorkflowRef string
	RunnerClass string // github-hosted | self-hosted | isolated-runner | witnessed-runner
}

// NewStatementV1 builds an in-toto Statement v1 with an agentattest v1
// predicate using upstream's predicate package. Output is JSON with sorted keys.
func NewStatementV1(b GitBinding, o StatementOptions) ([]byte, error) {
	if b.PatchSHA256 == "" || b.RepoURL == "" || b.BaseCommit == "" {
		return nil, errors.New("agentattest: binding requires repoUrl, baseCommit, and patch digest")
	}
	r := b.internal()
	pred := predicate.CreateV1(r, predicate.CreateOptions{
		AgentName: o.AgentName, AgentVersion: o.AgentVersion, CaptureMethod: o.CaptureMethod,
		ModelProvider: o.ModelProvider, ModelID: o.ModelID, Now: o.Now,
	})
	if o.InvocationKind != "" {
		pred["agent"].(map[string]any)["invocationKind"] = o.InvocationKind
	}
	if o.ExecutionType != "" {
		pred["environment"].(map[string]any)["executionType"] = o.ExecutionType
	}
	if o.VerificationLevel != "" {
		pred["verificationLevel"] = o.VerificationLevel
	}
	if o.Builder != nil {
		pred["environment"].(map[string]any)["builder"] = map[string]any{
			"id": o.Builder.ID, "type": o.Builder.Type, "workflowRef": o.Builder.WorkflowRef, "runnerClass": o.Builder.RunnerClass,
		}
	}
	if o.EvidenceStorage != "" {
		for _, e := range pred["evidence"].([]map[string]any) {
			e["storage"] = o.EvidenceStorage
		}
	}
	return json.Marshal(predicate.StatementForV1(r, pred))
}

// ContextOptions carry the verifier context fields beyond the binding. Verified
// fields must come from a verified signature, never from the statement.
type ContextOptions struct {
	RequiredLevel          string         // evidence-grade | policy-grade | high-assurance
	Verified               map[string]any // verifiedSigner, verifiedBuilderId, verifiedWorkflowRef, verifiedIssuer, ...
	Now                    time.Time      // explicit verification time
	FreshnessWindowSeconds int
	ExpectedRunID          string
	ExpectedPullRequest    int
}

// VerifierContext builds the out-of-band verifier context the same way
// upstream's `agentattest context` does (repoUrl, baseCommit, requiredLevel,
// patch subject), then merges verified identity and explicit time.
func VerifierContext(b GitBinding, o ContextOptions) ([]byte, error) {
	level := o.RequiredLevel
	if level == "" {
		level = "evidence-grade"
	}
	c := map[string]any{
		"repoUrl":       b.RepoURL,
		"baseCommit":    b.BaseCommit,
		"requiredLevel": level,
		"subjects": []map[string]any{{
			"name": "patch.diff", "algorithm": "sha256", "digest": b.PatchSHA256,
		}},
	}
	for k, v := range o.Verified {
		if !strings.HasPrefix(k, "verified") {
			return nil, fmt.Errorf("agentattest: %q is not a verified-context key", k)
		}
		c[k] = v
	}
	if !o.Now.IsZero() {
		c["now"] = o.Now.UTC().Format(time.RFC3339)
	}
	if o.FreshnessWindowSeconds > 0 {
		c["freshnessWindowSeconds"] = o.FreshnessWindowSeconds
	}
	if o.ExpectedRunID != "" {
		c["expectedRunId"] = o.ExpectedRunID
	}
	if o.ExpectedPullRequest > 0 {
		c["expectedPullRequestNumber"] = o.ExpectedPullRequest
	}
	return json.Marshal(c)
}

// TrustRoot mirrors upstream's signing.TrustRoot. Roots are PEM-encoded and
// parsed by internal/signing, the only package allowed to touch crypto types.
type TrustRoot struct {
	RootsPEM         []byte
	AllowedIssuers   []string
	RequireInclusion bool
	RekorRootHash    []byte
}

// VerifiedContext mirrors upstream's signing.VerifiedContext.
type VerifiedContext struct {
	VerifiedSigner      string
	VerifiedBuilderID   string
	VerifiedWorkflowRef string
	VerifiedIssuer      string
}

// VerifyBundle is upstream's Sigstore bundle check (signing.FromSigstoreBundle).
// Upstream validates certificates at the current time and does not verify
// RFC 3161 timestamps; consumers needing re-verification after certificate
// expiry must verify the bundle with a timestamp-aware verifier instead.
func VerifyBundle(ctx context.Context, bundleJSON []byte, tr TrustRoot) (VerifiedContext, []byte, error) {
	roots, err := signing.ParseCertificates(tr.RootsPEM)
	if err != nil || len(roots) == 0 {
		return VerifiedContext{}, nil, errors.New("agentattest: trust root has no PEM certificates")
	}
	vc, payload, err := signing.FromSigstoreBundle(ctx, bundleJSON, signing.TrustRoot{
		Roots: roots, AllowedIssuers: tr.AllowedIssuers, RequireInclusion: tr.RequireInclusion, RekorRootHash: tr.RekorRootHash,
	})
	if err != nil {
		return VerifiedContext{}, nil, err
	}
	return VerifiedContext{
		VerifiedSigner: vc.VerifiedSigner, VerifiedBuilderID: vc.VerifiedBuilderID,
		VerifiedWorkflowRef: vc.VerifiedWorkflowRef, VerifiedIssuer: vc.VerifiedIssuer,
	}, payload, nil
}
