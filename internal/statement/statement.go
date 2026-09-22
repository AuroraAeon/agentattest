package statement

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	TypeV1          = "https://in-toto.io/Statement/v1"
	PredicateTypeV0 = "https://agentattest.dev/predicate/v0"
	PredicateTypeV1 = "https://agentattest.dev/predicate/v1"
)

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type Document struct {
	Type          string          `json:"_type"`
	Subject       []Subject       `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

func Parse(raw []byte) (Document, map[string]any, error) {
	var doc Document
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return Document{}, nil, fmt.Errorf("decode statement: %w", err)
	}

	var generic map[string]any
	dec = json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return Document{}, nil, fmt.Errorf("decode generic statement: %w", err)
	}
	return doc, generic, nil
}

func PredicateMap(doc Document) (map[string]any, error) {
	var predicate map[string]any
	dec := json.NewDecoder(bytes.NewReader(doc.Predicate))
	dec.UseNumber()
	if err := dec.Decode(&predicate); err != nil {
		return nil, fmt.Errorf("decode predicate: %w", err)
	}
	return predicate, nil
}

func New(subjects []Subject, predicate map[string]any) map[string]any {
	return newWithType(PredicateTypeV0, subjects, predicate)
}

// NewV1 wraps subjects and a v1 predicate in an in-toto Statement v1 whose
// predicateType is https://agentattest.dev/predicate/v1.
func NewV1(subjects []Subject, predicate map[string]any) map[string]any {
	return newWithType(PredicateTypeV1, subjects, predicate)
}

func newWithType(predicateType string, subjects []Subject, predicate map[string]any) map[string]any {
	return map[string]any{
		"_type":         TypeV1,
		"subject":       subjects,
		"predicateType": predicateType,
		"predicate":     predicate,
	}
}
