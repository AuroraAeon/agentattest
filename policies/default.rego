package agentattest.default_policy

import rego.v1

default allow := false

level_rank := {"evidence-grade": 0, "policy-grade": 1, "high-assurance": 2}
transparency_forbidden_types := {"trace", "raw-prompt-ref", "raw-tool-output-ref"}

statement := object.get(input, "statement", {})
context := object.get(input, "context", {})
predicate := object.get(statement, "predicate", {})
agent := object.get(predicate, "agent", {})
environment := object.get(predicate, "environment", {})
builder := object.get(environment, "builder", {})
repo := object.get(predicate, "repo", {})
level := object.get(predicate, "verificationLevel", "")
required_level := object.get(context, "requiredLevel", "evidence-grade")
predicate_repo_url := object.get(repo, "url", "")
predicate_base_commit := object.get(repo, "baseCommit", "")
context_repo_url := object.get(context, "repoUrl", "")
context_base_commit := object.get(context, "baseCommit", "")
context_subjects := object.get(context, "subjects", [])
statement_subjects := object.get(statement, "subject", [])
evidence_refs := object.get(predicate, "evidence", [])
witnesses := object.get(environment, "witnesses", [])
extensions := object.get(predicate, "extensions", {})

allow if {
	count(deny) == 0
}

failure_codes contains code if {
	some item in deny
	code := item.code
}

deny contains {"code": "level_below_required", "message": sprintf("verificationLevel %q is below requiredLevel %q", [level, required_level])} if {
	level_rank[level] < level_rank[required_level]
}

deny contains {"code": "repo_url_mismatch", "message": sprintf("repo URL mismatch: predicate=%q context=%q", [predicate_repo_url, context_repo_url])} if {
	predicate_repo_url != context_repo_url
}

deny contains {"code": "base_commit_mismatch", "message": sprintf("base commit mismatch: predicate=%q context=%q", [predicate_base_commit, context_base_commit])} if {
	predicate_base_commit != context_base_commit
}

deny contains {"code": "subject_digest_mismatch", "message": "no current subject digests were provided"} if {
	count(context_subjects) == 0
}

deny contains {"code": "subject_digest_mismatch", "message": "context.subjects contains duplicate subject names"} if {
	context_subject_names := [subject.name | some subject in context_subjects]
	context_subject_name_set := {subject.name | some subject in context_subjects}
	count(context_subject_names) != count(context_subject_name_set)
}

deny contains {"code": "subject_digest_mismatch", "message": sprintf("subject digest mismatch for %q", [expected.name])} if {
	some expected in context_subjects
	not statement_subject_matches(expected)
}

deny contains {"code": "subject_digest_mismatch", "message": sprintf("unexpected statement subject %q", [subject.name])} if {
	some subject in statement_subjects
	not context_subject_matches(subject)
}

deny contains {"code": "privacy_violation", "message": sprintf("%s evidence must not be written to a transparency log", [evidence.type])} if {
	some evidence in evidence_refs
	evidence.type in transparency_forbidden_types
	evidence.storage == "transparency-log"
}

deny contains {"code": "privacy_violation", "message": "non-public evidence must not use transparency-log storage"} if {
	some evidence in evidence_refs
	evidence.storage == "transparency-log"
	evidence.visibility != "public"
}

deny contains {"code": "privacy_violation", "message": sprintf("extension %q must not be public", [key])} if {
	some key
	extension := extensions[key]
	extension.visibility == "public"
}

deny contains {"code": "level_escalation", "message": sprintf("%s cannot rely on local-only evidence", [level])} if {
	policy_or_higher
	some evidence in evidence_refs
	evidence.storage == "local-only"
}

deny contains {"code": "level_escalation", "message": "policy-grade and high-assurance cannot be claimed from local execution"} if {
	policy_or_higher
	environment.executionType == "local"
}

deny contains {"code": "level_escalation", "message": sprintf("%s requires a builder object", [level])} if {
	policy_or_higher
	count(builder) == 0
}

deny contains {"code": "builder_identity_mismatch", "message": "verified builder identity is required"} if {
	policy_or_higher
	object.get(context, "verifiedBuilderId", "") == ""
}

deny contains {"code": "builder_identity_mismatch", "message": "predicate builder id does not match verified builder id"} if {
	policy_or_higher
	verified_builder := object.get(context, "verifiedBuilderId", "")
	verified_builder != ""
	object.get(builder, "id", "") != verified_builder
}

deny contains {"code": "builder_identity_mismatch", "message": "verified workflow ref is required"} if {
	policy_or_higher
	object.get(context, "verifiedWorkflowRef", "") == ""
}

deny contains {"code": "builder_identity_mismatch", "message": "predicate workflowRef does not match verified workflow ref"} if {
	policy_or_higher
	verified_workflow := object.get(context, "verifiedWorkflowRef", "")
	verified_workflow != ""
	object.get(builder, "workflowRef", "") != verified_workflow
}

deny contains {"code": "builder_identity_mismatch", "message": "verified issuer is required"} if {
	policy_or_higher
	object.get(context, "verifiedIssuer", "") == ""
}

deny contains {"code": "signer_identity_mismatch", "message": "verified signer is required"} if {
	policy_or_higher
	object.get(context, "verifiedSigner", "") == ""
}

deny contains {"code": "signer_identity_mismatch", "message": "declared agent identity does not match verified signer"} if {
	declared := object.get(agent, "declaredIdentity", "")
	declared != ""
	object.get(context, "verifiedSigner", "") != declared
}

deny contains {"code": "builder_identity_mismatch", "message": "self-hosted GitHub runners require explicit verifier opt-in"} if {
	policy_or_higher
	environment.executionType == "github-actions"
	object.get(builder, "runnerClass", "") == "self-hosted"
	object.get(context, "allowSelfHostedRunners", false) != true
}

deny contains {"code": "builder_identity_mismatch", "message": "high-assurance isolated runner is not verifier-allowlisted"} if {
	level == "high-assurance"
	not array_contains(object.get(context, "allowedIsolatedRunners", []), object.get(builder, "id", ""))
}

deny contains {"code": "level_escalation", "message": "high-assurance requires isolated-runner or witnessed-runner execution"} if {
	level == "high-assurance"
	not environment.executionType in {"isolated-runner", "witnessed-runner"}
}

deny contains {"code": "level_escalation", "message": "high-assurance requires at least one witness"} if {
	level == "high-assurance"
	count(witnesses) == 0
}

deny contains {"code": "transparency_verification_failed", "message": sprintf("witness %q is not backed by verified context", [witness.id])} if {
	some witness in witnesses
	not verified_witness_matches(witness)
}

deny contains {"code": "level_escalation", "message": "high-assurance requires approved-before-merge approval state"} if {
	level == "high-assurance"
	predicate.humanApproval.state != "approved-before-merge"
}

deny contains {"code": "missing_evidence", "message": "high-assurance requires approval evidence"} if {
	level == "high-assurance"
	not has_evidence_type("approval")
}

deny contains {"code": "missing_evidence", "message": "high-assurance approval must be verified out of band"} if {
	level == "high-assurance"
	object.get(context, "verifiedApproval", false) != true
}

deny contains {"code": "missing_evidence", "message": "verified approval digest must match approval evidence"} if {
	level == "high-assurance"
	object.get(context, "verifiedApproval", false) == true
	not approval_evidence_matches_verified_digest
}

deny contains {"code": "replay_detected", "message": "pull request number does not match verifier context"} if {
	expected_pr := object.get(context, "expectedPullRequestNumber", 0)
	expected_pr > 0
	actual_pr := object.get(object.get(repo, "pullRequest", {}), "number", 0)
	actual_pr != expected_pr
}

deny contains {"code": "replay_detected", "message": "runId does not match verifier context"} if {
	expected_run := object.get(context, "expectedRunId", "")
	expected_run != ""
	object.get(predicate, "runId", "") != expected_run
}

deny contains {"code": "stale_attestation", "message": "attestation is outside the configured freshness window"} if {
	window_seconds := object.get(context, "freshnessWindowSeconds", 0)
	window_seconds > 0
	object.get(context, "now", "") == ""
}

deny contains {"code": "stale_attestation", "message": "attestation is outside the configured freshness window"} if {
	window_seconds := object.get(context, "freshnessWindowSeconds", 0)
	window_seconds > 0
	object.get(context, "now", "") != ""
	now := time.parse_rfc3339_ns(context.now)
	finished := time.parse_rfc3339_ns(predicate.timestamps.finishedAt)
	now - finished > window_seconds * 1000000000
}

policy_or_higher if {
	level_rank[level] >= level_rank["policy-grade"]
}

statement_subject_matches(expected) if {
	some subject in statement_subjects
	subject.name == expected.name
	subject.digest[expected.algorithm] == expected.digest
}

context_subject_matches(subject) if {
	some expected in context_subjects
	expected.name == subject.name
	subject.digest[expected.algorithm] == expected.digest
}

has_evidence_type(kind) if {
	some evidence in evidence_refs
	evidence.type == kind
}

verified_witness_matches(witness) if {
	some verified in object.get(context, "verifiedWitnesses", [])
	verified.type == witness.type
	verified.digest.sha256 == witness.digest.sha256
}

approval_evidence_matches_verified_digest if {
	expected := object.get(object.get(context, "verifiedApprovalDigest", {}), "sha256", "")
	expected != ""
	some evidence in evidence_refs
	evidence.type == "approval"
	evidence.digest.sha256 == expected
}

array_contains(values, value) if {
	some current in values
	current == value
}
