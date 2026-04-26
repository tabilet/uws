package uws1

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaParityEntry binds a Go type that carries x-* extensions to the $def
// that describes it in versions/1.0.0.json, and to the package-level knownFields list its
// UnmarshalJSON uses to reject unknown properties. Adding a new such type is
// the one place that must be kept in sync by hand; the test below catches
// every other drift direction automatically.
//
// defName == "" means the schema root (the Document struct).
type schemaParityEntry struct {
	label       string
	defName     string
	goType      reflect.Type
	knownFields []string
}

func schemaParityEntries() []schemaParityEntry {
	return []schemaParityEntry{
		{label: "Document", defName: "", goType: reflect.TypeOf(Document{}), knownFields: documentKnownFields},
		{label: "Info", defName: "info", goType: reflect.TypeOf(Info{}), knownFields: infoKnownFields},
		{label: "SourceDescription", defName: "source-description-object", goType: reflect.TypeOf(SourceDescription{}), knownFields: sourceDescriptionKnownFields},
		{label: "Operation", defName: "operation-object", goType: reflect.TypeOf(Operation{}), knownFields: operationKnownFields},
		{label: "Workflow", defName: "workflow-object", goType: reflect.TypeOf(Workflow{}), knownFields: workflowKnownFields},
		{label: "Step", defName: "step-object", goType: reflect.TypeOf(Step{}), knownFields: stepKnownFields},
		{label: "Case", defName: "case-object", goType: reflect.TypeOf(Case{}), knownFields: caseKnownFields},
		{label: "Trigger", defName: "trigger-object", goType: reflect.TypeOf(Trigger{}), knownFields: triggerKnownFields},
		{label: "TriggerRoute", defName: "trigger-route-object", goType: reflect.TypeOf(TriggerRoute{}), knownFields: triggerRouteKnownFields},
		{label: "ParamSchema", defName: "param-schema-object", goType: reflect.TypeOf(ParamSchema{}), knownFields: paramSchemaKnownFields},
		{label: "StructuralResult", defName: "structural-result-object", goType: reflect.TypeOf(StructuralResult{}), knownFields: structuralResultKnownFields},
		{label: "Components", defName: "components-object", goType: reflect.TypeOf(Components{}), knownFields: componentsKnownFields},
		{label: "Criterion", defName: "criterion-object", goType: reflect.TypeOf(Criterion{}), knownFields: criterionKnownFields},
		{label: "FailureAction", defName: "failure-action-object", goType: reflect.TypeOf(FailureAction{}), knownFields: failureActionKnownFields},
		{label: "SuccessAction", defName: "success-action-object", goType: reflect.TypeOf(SuccessAction{}), knownFields: successActionKnownFields},
	}
}

// TestSchemaParity_StructTagsMatchKnownFields ensures every type with an
// Extensions map keeps its struct JSON tags and its knownFields list in sync.
// A mismatch means rejectUnknownFields would either reject valid documents or
// silently accept invalid ones.
func TestSchemaParity_StructTagsMatchKnownFields(t *testing.T) {
	for _, entry := range schemaParityEntries() {
		t.Run(entry.label, func(t *testing.T) {
			gotTags := jsonFieldTags(t, entry.goType)
			assert.ElementsMatch(t, entry.knownFields, gotTags,
				"struct %s JSON tags do not match its knownFields list", entry.label)

			require.Contains(t, namedFields(entry.goType), "Extensions",
				"type %s is in parity list but has no Extensions field", entry.label)
		})
	}
}

// TestSchemaParity_KnownFieldsMatchSchema ensures every parity-tracked type's
// knownFields list is exactly the set of non-extension properties declared by
// its $def in versions/1.0.0.json. A drift in either direction fails this test: a new
// schema property without a Go equivalent, or a Go known field without a
// schema property.
func TestSchemaParity_KnownFieldsMatchSchema(t *testing.T) {
	schema := loadSchemaDoc(t)

	for _, entry := range schemaParityEntries() {
		t.Run(entry.label, func(t *testing.T) {
			schemaProps := schemaPropertyNames(t, schema, entry.defName)
			nonExtensionSchemaProps := dropExtensionKeys(schemaProps)

			assert.ElementsMatch(t, entry.knownFields, nonExtensionSchemaProps,
				"%s knownFields diverge from schema %q properties", entry.label, entry.defName)
		})
	}
}

// TestSchemaParity_DefCoverageIsExhaustive fails when versions/1.0.0.json grows a $def
// that no parity entry tracks. This is the tripwire for adding a new type
// without wiring it through the extension machinery.
func TestSchemaParity_DefCoverageIsExhaustive(t *testing.T) {
	schema := loadSchemaDoc(t)
	defs, ok := schema["$defs"].(map[string]any)
	require.True(t, ok, "schema $defs is not an object")

	tracked := map[string]bool{}
	for _, entry := range schemaParityEntries() {
		if entry.defName != "" {
			tracked[entry.defName] = true
		}
	}
	// Meta defs that describe JSON Schema plumbing, not UWS object shapes.
	tracked["specification-extensions"] = true
	tracked["structural-type-constraints"] = true
	// request-binding-object is a bag of free-form locations backed by
	// map[string]any on Operation, not a dedicated Go type.
	tracked["request-binding-object"] = true

	var untracked []string
	for name := range defs {
		if !tracked[name] {
			untracked = append(untracked, name)
		}
	}
	sort.Strings(untracked)
	assert.Empty(t, untracked,
		"versions/1.0.0.json declares $defs that no schemaParityEntries covers: %v", untracked)
}

func jsonFieldTags(t *testing.T, typ reflect.Type) []string {
	t.Helper()
	if typ.Kind() != reflect.Struct {
		t.Fatalf("type %s is not a struct", typ)
	}
	var tags []string
	var collect func(reflect.Type)
	collect = func(current reflect.Type) {
		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				collect(field.Type)
				continue
			}
			tag, ok := field.Tag.Lookup("json")
			if !ok {
				continue
			}
			name, _, _ := strings.Cut(tag, ",")
			if name == "" || name == "-" {
				continue
			}
			tags = append(tags, name)
		}
	}
	collect(typ)
	return tags
}

func namedFields(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
	}
	return names
}

func loadSchemaDoc(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../versions/1.0.0.json")
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	return schema
}

// schemaPropertyNames returns the property names of a $def. When defName is
// empty, it returns the root document's properties.
func schemaPropertyNames(t *testing.T, schema map[string]any, defName string) []string {
	t.Helper()
	obj := schema
	if defName != "" {
		defs, ok := schema["$defs"].(map[string]any)
		require.True(t, ok, "schema has no $defs")
		entry, ok := defs[defName].(map[string]any)
		require.True(t, ok, "schema $defs has no %q", defName)
		obj = entry
	}
	props, ok := obj["properties"].(map[string]any)
	require.Truef(t, ok, "schema %q has no properties object", fmt.Sprintf("$defs/%s", defName))
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func dropExtensionKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.HasPrefix(k, "x-") {
			continue
		}
		out = append(out, k)
	}
	return out
}
