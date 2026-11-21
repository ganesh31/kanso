package api

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"kanso/internal/patterns"
)

// evidenceSchemaName is the OpenAPI component name of the Evidence union.
const evidenceSchemaName = "Evidence"

// schemaPrefix is where huma's default registry puts named schemas.
const schemaPrefix = "#/components/schemas/"

// Evidence wraps a patterns.Evidence so the API can describe it as a
// oneOf over every evidence variant, discriminated by "type" (ADR 0003).
// It marshals as the wrapped value.
type Evidence struct {
	patterns.Evidence
}

// MarshalJSON writes the wrapped evidence.
func (e Evidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Evidence)
}

// Schema implements huma.SchemaProvider. It registers each variant as its
// own component and the union as the "Evidence" component, then returns a
// reference to the union.
func (Evidence) Schema(r huma.Registry) *huma.Schema {
	union := &huma.Schema{
		Description:   "Evidence behind one condition's verdict, discriminated by type.",
		Discriminator: &huma.Discriminator{PropertyName: "type", Mapping: map[string]string{}},
	}
	for _, ev := range patterns.Evidences() {
		ref := r.Schema(reflect.TypeOf(ev), true, "")
		union.OneOf = append(union.OneOf, &huma.Schema{Ref: ref.Ref})
		union.Discriminator.Mapping[ev.EvidenceType()] = ref.Ref
	}
	r.Map()[evidenceSchemaName] = union
	return &huma.Schema{Ref: schemaPrefix + evidenceSchemaName}
}
