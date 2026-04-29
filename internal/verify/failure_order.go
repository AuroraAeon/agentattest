package verify

import "sort"

var failurePriority = map[string]int{
	"unsupported_statement_type":       10,
	"predicate_type_mismatch":          20,
	"unsupported_predicate_version":    30,
	"schema_invalid":                   40,
	"signature_invalid":                50,
	"transparency_verification_failed": 60,
	"subject_digest_mismatch":          70,
	"repo_url_mismatch":                80,
	"base_commit_mismatch":             90,
	"signer_identity_mismatch":         100,
	"builder_identity_mismatch":        110,
	"replay_detected":                  120,
	"privacy_violation":                130,
	"level_escalation":                 140,
	"level_below_required":             150,
	"missing_evidence":                 160,
	"stale_attestation":                170,
	"policy_eval_error":                180,
	"cache_untrusted":                  190,
}

// OrderFailureCodes returns deterministic user-visible failure-code order.
// Rego deny sets are unordered, so the Go verifier owns presentation order.
func OrderFailureCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	ordered := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		ordered = append(ordered, code)
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		leftPriority, leftKnown := failurePriority[ordered[i]]
		rightPriority, rightKnown := failurePriority[ordered[j]]
		switch {
		case leftKnown && rightKnown:
			return leftPriority < rightPriority
		case leftKnown:
			return true
		case rightKnown:
			return false
		default:
			return ordered[i] < ordered[j]
		}
	})

	return ordered
}
