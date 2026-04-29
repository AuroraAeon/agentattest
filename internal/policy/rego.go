package policy

import (
	"context"

	"github.com/open-policy-agent/opa/v1/rego"
)

func FailureCodes(ctx context.Context, moduleName string, module []byte, input map[string]any) ([]string, error) {
	results, err := rego.New(
		rego.Query("data.agentattest.default_policy.failure_codes"),
		rego.Module(moduleName, string(module)),
		rego.Input(input),
	).Eval(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return nil, nil
	}

	var codes []string
	collectStrings(results[0].Expressions[0].Value, &codes)
	return codes, nil
}

func collectStrings(value any, out *[]string) {
	switch typed := value.(type) {
	case string:
		*out = append(*out, typed)
	case []any:
		for _, item := range typed {
			collectStrings(item, out)
		}
	case map[string]any:
		for _, item := range typed {
			collectStrings(item, out)
		}
	}
}
