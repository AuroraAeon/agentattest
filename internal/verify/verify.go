package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"agentattest.dev/agentattest/internal/contracts"
	"agentattest.dev/agentattest/internal/policy"
	"agentattest.dev/agentattest/internal/statement"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	cuejson "cuelang.org/go/encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Options struct {
	Contracts contracts.Files
}

func Predicate(ctx context.Context, statementJSON, contextJSON []byte, opts Options) Result {
	files := opts.Contracts
	if files.Root == "" {
		located, err := contracts.Locate("")
		if err != nil {
			return Result{FailureCodes: []string{"schema_invalid"}}
		}
		files = located
	}

	doc, statementInput, err := statement.Parse(statementJSON)
	if err != nil {
		return finish("", []string{"schema_invalid"})
	}
	predicateMap, err := statement.PredicateMap(doc)
	if err != nil {
		return finish("", []string{"schema_invalid"})
	}
	level := stringValue(predicateMap, "verificationLevel")

	if codes := phase01(doc, predicateMap); len(codes) > 0 {
		return finish(level, codes)
	}

	schemaPath, cuePath, cueDef, ok := contractForVersion(files, doc.PredicateType)
	if !ok {
		return finish(level, []string{"predicate_type_mismatch"})
	}

	if err := validateJSONSchema(schemaPath, doc.Predicate); err != nil {
		return finish(level, []string{"schema_invalid"})
	}
	if err := validateJSONSchema(files.ContextSchema, contextJSON); err != nil {
		return finish(level, []string{"schema_invalid"})
	}

	if err := validateCUE(cuePath, cueDef, doc.Predicate); err != nil {
		return finish(level, []string{"schema_invalid"})
	}

	var contextInput map[string]any
	dec := json.NewDecoder(bytes.NewReader(contextJSON))
	dec.UseNumber()
	if err := dec.Decode(&contextInput); err != nil {
		return finish(level, []string{"schema_invalid"})
	}

	module, err := contracts.Read(files.DefaultPolicy)
	if err != nil {
		return finish(level, []string{"policy_eval_error"})
	}
	codes, err := policy.FailureCodes(ctx, filepath.ToSlash(contracts.DefaultPolicyRel), module, map[string]any{
		"statement": statementInput,
		"context":   contextInput,
	})
	if err != nil {
		return finish(level, []string{"policy_eval_error"})
	}

	return finish(level, codes)
}

func phase01(doc statement.Document, predicate map[string]any) []string {
	var codes []string
	if doc.Type != statement.TypeV1 {
		codes = append(codes, "unsupported_statement_type")
	}
	versionFromType := ""
	switch doc.PredicateType {
	case statement.PredicateTypeV0:
		versionFromType = "v0"
	case statement.PredicateTypeV1:
		versionFromType = "v1"
	default:
		codes = append(codes, "predicate_type_mismatch")
	}
	declared := stringValue(predicate, "predicateVersion")
	switch declared {
	case "v0", "v1":
		if versionFromType != "" && declared != versionFromType {
			codes = append(codes, "unsupported_predicate_version")
		}
	default:
		codes = append(codes, "unsupported_predicate_version")
	}
	return codes
}

// contractForVersion selects the predicate schema file, CUE file, and CUE
// definition for a statement's predicateType. It returns ok=false for any
// predicateType that is not a known agentattest contract version.
func contractForVersion(files contracts.Files, predicateType string) (schemaPath, cuePath, cueDef string, ok bool) {
	switch predicateType {
	case statement.PredicateTypeV0:
		return files.PredicateSchema, files.PredicateCUE, "#AgentProvenanceV0", true
	case statement.PredicateTypeV1:
		return files.PredicateSchemaV1, files.PredicateCUEV1, "#AgentProvenanceV1", true
	default:
		return "", "", "", false
	}
}

func finish(level string, codes []string) Result {
	ordered := OrderFailureCodes(codes)
	return Result{
		Valid:        len(ordered) == 0,
		Level:        level,
		FailureCodes: ordered,
	}
}

func stringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func validateJSONSchema(schemaPath string, document []byte) error {
	var instance any
	dec := json.NewDecoder(bytes.NewReader(document))
	dec.UseNumber()
	if err := dec.Decode(&instance); err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	schema, err := compiler.Compile(fileURL(schemaPath))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func validateCUE(cuePath, cueDef string, predicateJSON []byte) error {
	ctx := cuecontext.New()
	instances := load.Instances([]string{cuePath}, &load.Config{Dir: filepath.Dir(cuePath)})
	if len(instances) != 1 {
		return fmt.Errorf("expected one CUE instance, got %d", len(instances))
	}
	contract := ctx.BuildInstance(instances[0])
	if err := contract.Err(); err != nil {
		return err
	}
	schema := contract.LookupPath(cue.MakePath(cue.Def(cueDef)))
	if !schema.Exists() {
		return fmt.Errorf("CUE definition %s not found", cueDef)
	}

	expr, err := cuejson.Extract("predicate.json", predicateJSON)
	if err != nil {
		return err
	}
	value := ctx.BuildExpr(expr)
	if err := value.Err(); err != nil {
		return err
	}
	return schema.Unify(value).Validate(cue.Concrete(true))
}

func fileURL(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		clean = strings.ReplaceAll(clean, `\`, `/`)
		if !strings.HasPrefix(clean, "/") {
			clean = "/" + clean
		}
		return (&url.URL{Scheme: "file", Path: clean}).String()
	}
	return (&url.URL{Scheme: "file", Path: clean}).String()
}
