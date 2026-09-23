package agentattest

# Intentionally syntactically broken policy module. Used by
# TestPolicyEvalErrorOnBrokenPolicy to pin the stable policy_eval_error code
# when the Rego policy cannot be evaluated. Never loaded by the verifier.
default failure_codes := {{{
this is not valid rego
