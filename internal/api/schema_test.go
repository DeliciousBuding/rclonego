package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContractHasResolvedTypesAndExplicitSuccessfulResponses(t *testing.T) {
	var document struct {
		Paths      map[string]map[string]map[string]any `json:"paths"`
		Components struct {
			Schemas map[string]map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(Contract(), &document); err != nil {
		t.Fatal(err)
	}
	var verify func(any)
	verify = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			if reference, ok := item["$ref"].(string); ok {
				name := strings.TrimPrefix(reference, "#/components/schemas/")
				if _, exists := document.Components.Schemas[name]; !exists {
					t.Fatalf("unresolved contract type %s", reference)
				}
			}
			for _, child := range item {
				verify(child)
			}
		case []any:
			for _, child := range item {
				verify(child)
			}
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(Contract(), &raw); err != nil {
		t.Fatal(err)
	}
	verify(raw)
	for path, methods := range document.Paths {
		for method, operation := range methods {
			responses := operation["responses"].(map[string]any)
			for status, response := range responses {
				if !strings.HasPrefix(status, "2") {
					continue
				}
				content := response.(map[string]any)["content"].(map[string]any)
				for _, media := range content {
					schema := media.(map[string]any)["schema"].(map[string]any)
					if len(schema) == 0 {
						t.Fatalf("%s %s lacks a typed %s response", method, path, status)
					}
				}
			}
		}
	}
	taskGrant := document.Components.Schemas["TaskTokenRequest"]
	required := taskGrant["required"].([]any)
	if len(required) != 2 || required[0] != "operations" || required[1] != "paths" {
		t.Fatal("task grant contract must require operations and paths")
	}
	if _, exists := document.Paths["/api/v1/events/watch"]["get"]["responses"].(map[string]any)["410"]; !exists {
		t.Fatal("event watch contract must describe cursor expiration")
	}
}
