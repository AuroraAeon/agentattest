package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"agentattest.dev/agentattest/internal/contracts"
	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/signing"
	"agentattest.dev/agentattest/internal/verify"
)

// buildVerifierContext derives a minimal out-of-band verifier context from the
// current repo state: the repo URL, base commit, the patch subject, and the
// required level. Callers (or the GitHub Action) merge verified-identity fields
// on top for policy-grade and above.
func buildVerifierContext(result gitbind.Result, requiredLevel string) map[string]any {
	if requiredLevel == "" {
		requiredLevel = "evidence-grade"
	}
	return map[string]any{
		"repoUrl":       result.RepoURL,
		"baseCommit":    result.BaseCommit,
		"requiredLevel": requiredLevel,
		"subjects": []map[string]any{
			{
				"name":      "patch.diff",
				"algorithm": result.Patch.Algorithm,
				"digest":    result.Patch.Digest,
			},
		},
	}
}

func runContext(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository directory")
	requiredLevel := fs.String("required-level", "evidence-grade", "minimum verification level (evidence-grade|policy-grade|high-assurance)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := gitbind.Compute(ctx, *repo)
	if err != nil {
		return err
	}
	return writeJSON(stdout, buildVerifierContext(result, *requiredLevel))
}

func runSummary(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statementPath := fs.String("statement", "", "statement JSON path")
	contextPath := fs.String("context", "", "verifier context JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *statementPath == "" || *contextPath == "" {
		return errors.New("--statement and --context are required")
	}
	statementJSON, err := os.ReadFile(*statementPath)
	if err != nil {
		return err
	}
	contextJSON, err := os.ReadFile(*contextPath)
	if err != nil {
		return err
	}
	files, err := contracts.Locate("")
	if err != nil {
		return err
	}

	statement, err := decodeObject(statementJSON)
	if err != nil {
		return fmt.Errorf("decode statement: %w", err)
	}
	contextMap, err := decodeObject(contextJSON)
	if err != nil {
		return fmt.Errorf("decode context: %w", err)
	}

	result := verify.Predicate(ctx, statementJSON, contextJSON, verify.Options{Contracts: files})
	_, err = io.WriteString(stdout, renderSummary(result, statement, contextMap))
	return err
}

func decodeObject(data []byte) (map[string]any, error) {
	var obj map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// renderSummary produces a deterministic, privacy-safe Markdown summary for a PR
// check or $GITHUB_STEP_SUMMARY. It shows verification status, level, subject
// digest prefixes, repo/base match, verified identity, privacy presence flags,
// and the v1 capture descriptor. It never emits raw prompts, tool outputs, or
// trace content — only digests, presence booleans, and identity strings.
func renderSummary(result verify.Result, statement, context map[string]any) string {
	predicate, _ := statement["predicate"].(map[string]any)
	var b strings.Builder

	if result.Valid {
		fmt.Fprintf(&b, "## Agent Provenance: valid (%s)\n\n", orNA(result.Level))
	} else {
		b.WriteString("## Agent Provenance: invalid\n\n")
	}

	b.WriteString("**Subjects**\n")
	if subjects, ok := statement["subject"].([]any); ok && len(subjects) > 0 {
		for _, s := range subjects {
			sm, _ := s.(map[string]any)
			name, _ := sm["name"].(string)
			fmt.Fprintf(&b, "- `%s` = `%s`\n", orNA(name), shortHash(digestHex(sm)))
		}
	} else {
		b.WriteString("- (none)\n")
	}

	repo, _ := predicate["repo"].(map[string]any)
	repoURL, _ := repo["url"].(string)
	base, _ := repo["baseCommit"].(string)
	ctxRepoURL, _ := context["repoUrl"].(string)
	ctxBase, _ := context["baseCommit"].(string)
	fmt.Fprintf(&b, "\n**Repository**: %s\n", orNA(repoURL))
	fmt.Fprintf(&b, "- repo URL matches context: %s\n", yesNo(repoURL != "" && repoURL == ctxRepoURL))
	fmt.Fprintf(&b, "- base commit `%s` matches context: %s\n", shortHash(base), yesNo(base != "" && base == ctxBase))

	signer, _ := context["verifiedSigner"].(string)
	builder, _ := context["verifiedBuilderId"].(string)
	issuer, _ := context["verifiedIssuer"].(string)
	b.WriteString("\n**Verified identity** (from out-of-band context)\n")
	fmt.Fprintf(&b, "- signer: %s\n", valueOrNA(signer))
	fmt.Fprintf(&b, "- builder: %s\n", valueOrNA(builder))
	fmt.Fprintf(&b, "- issuer: %s\n", valueOrNA(issuer))

	privacy, _ := predicate["privacy"].(map[string]any)
	redaction, _ := privacy["redactionPolicy"].(string)
	fmt.Fprintf(&b, "\n**Privacy**\n")
	fmt.Fprintf(&b, "- redaction policy: %s\n", orNA(redaction))
	fmt.Fprintf(&b, "- raw prompt stored: %s; raw tool output stored: %s\n",
		yesNo(rawStored(privacy, "rawPrompt")), yesNo(rawStored(privacy, "rawToolOutputs")))

	if capture, ok := predicate["capture"].(map[string]any); ok {
		method, _ := capture["method"].(string)
		harness, _ := capture["harness"].(string)
		if harness != "" {
			fmt.Fprintf(&b, "\n**Capture**: %s (%s)\n", orNA(method), harness)
		} else {
			fmt.Fprintf(&b, "\n**Capture**: %s\n", orNA(method))
		}
	}

	if !result.Valid {
		fmt.Fprintf(&b, "\n**Failure codes**: %s\n", strings.Join(result.FailureCodes, ", "))
	}
	return b.String()
}

func digestHex(subject map[string]any) string {
	dg, _ := subject["digest"].(map[string]any)
	sha, _ := dg["sha256"].(string)
	return sha
}

func rawStored(privacy map[string]any, key string) bool {
	re, _ := privacy[key].(map[string]any)
	stored, _ := re["stored"].(bool)
	return stored
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

func valueOrNA(s string) string {
	if s == "" {
		return "not provided"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func shortHash(s string) string {
	if s == "" {
		return "n/a"
	}
	if len(s) <= 12 {
		return s
	}
	return s[:8] + "…" + s[len(s)-4:]
}

// runVerifyBundle verifies a Sigstore / GitHub attestation bundle end to end: it
// extracts the DSSE envelope + certificate chain, re-verifies the chain against a
// trusted root, derives the verified identity context, binds it to the current
// repo subjects, and runs the 5-phase verifier on the decoded statement. It fails
// closed on any step.
func runVerifyBundle(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify bundle", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bundle := fs.String("bundle", "", "Sigstore bundle or gh attestation verify JSON path")
	trustRoot := fs.String("trust-root", "", "PEM file with trusted root CA certificate(s); when omitted, the Sigstore community root is fetched via TUF")
	repo := fs.String("repo", ".", "repository directory")
	requiredLevel := fs.String("required-level", "policy-grade", "minimum verification level")
	requireInclusion := fs.Bool("require-inclusion", false, "fail closed if the bundle has no Rekor inclusion proof")
	rekorRoot := fs.String("rekor-root", "", "hex sha256 of the trusted Rekor log root to bind the inclusion proof to")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if *bundle == "" {
		fmt.Fprintln(stderr, "error: --bundle is required (--trust-root is optional; omit it to use the Sigstore community root via TUF)")
		return 2
	}

	bundleJSON, err := os.ReadFile(*bundle)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	var trustPEM []byte
	if *trustRoot != "" {
		trustPEM, err = os.ReadFile(*trustRoot)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	}
	roots, err := signing.LoadTrustRoot(ctx, signing.TrustRootOptions{ExplicitPEM: trustPEM})
	if err != nil {
		return signatureFailure(stdout, stderr, fmt.Errorf("load trust root: %w", err))
	}

	trust := signing.TrustRoot{Roots: roots, RequireInclusion: *requireInclusion}
	if *rekorRoot != "" {
		rootHash, err := hex.DecodeString(*rekorRoot)
		if err != nil {
			fmt.Fprintln(stderr, "error: --rekor-root must be a hex sha256:", err)
			return 2
		}
		trust.RekorRootHash = rootHash
	}

	verified, payload, err := signing.FromSigstoreBundle(ctx, bundleJSON, trust)
	if err != nil {
		return signatureFailure(stdout, stderr, fmt.Errorf("verify bundle: %w", err))
	}

	result, err := gitbind.Compute(ctx, *repo)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	contextMap := buildVerifierContext(result, *requiredLevel)
	for k, v := range verified.ToContextMap() {
		contextMap[k] = v
	}
	contextJSON, err := json.Marshal(contextMap)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}

	files, err := contracts.Locate("")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	res := verify.Predicate(ctx, payload, contextJSON, verify.Options{Contracts: files})
	if err := writeJSON(stdout, res); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if !res.Valid {
		return 1
	}
	return 0
}

// signatureFailure reports a bundle/trust-root verification failure as a
// structured verifier result (stable code signature_invalid) on stdout, with
// the human-readable cause on stderr. Machine consumers of `verify bundle`
// previously could not distinguish signature failures from I/O errors.
func signatureFailure(stdout, stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "error:", err)
	res := verify.Result{Valid: false, FailureCodes: []string{"signature_invalid"}}
	if writeErr := writeJSON(stdout, res); writeErr != nil {
		fmt.Fprintln(stderr, "error:", writeErr)
	}
	return 1
}
