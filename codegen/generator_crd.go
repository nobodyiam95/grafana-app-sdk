package codegen

import (
	"fmt"

	"cuelang.org/go/cue"
	cueopenapi "cuelang.org/go/encoding/openapi"
	cueyaml "cuelang.org/go/pkg/encoding/yaml"
	"github.com/grafana/codejen"
	"github.com/grafana/thema"
	"github.com/grafana/thema/encoding/openapi"
	goyaml "gopkg.in/yaml.v3"

	"github.com/grafana/grafana-app-sdk/k8s"
	"github.com/grafana/grafana-app-sdk/kindsys"
)

// CRDOutputEncoder is a function which marshals an object into a desired output format
type CRDOutputEncoder func(any) ([]byte, error)

type crdGenerator struct {
	outputEncoder   CRDOutputEncoder
	outputExtension string
}

func (*crdGenerator) JennyName() string {
	return "CRD Generator"
}

func (c *crdGenerator) Generate(decl kindsys.Custom) (*codejen.File, error) {
	meta := decl.Def().Properties
	lin := decl.Lineage()

	// We need to go through every schema, as they all have to be defined in the CRD
	sch, err := lin.Schema(thema.SV(0, 0))
	if err != nil {
		return nil, err
	}

	resource := customResourceDefinition{
		APIVersion: "apiextensions.k8s.io/v1",
		Kind:       "CustomResourceDefinition",
		Metadata: customResourceDefinitionMetadata{
			Name: fmt.Sprintf("%s.%s", meta.PluralMachineName, decl.Def().Properties.CRD.Group),
		},
		Spec: k8s.CustomResourceDefinitionSpec{
			Group: decl.Def().Properties.CRD.Group,
			Scope: decl.Def().Properties.CRD.Scope,
			Names: k8s.CustomResourceDefinitionSpecNames{
				Kind:   meta.Name,
				Plural: meta.PluralMachineName,
			},
			Versions: make([]k8s.CustomResourceDefinitionSpecVersion, 0),
		},
	}
	latest := lin.Latest().Version()

	for sch != nil {
		ver, err := schemaToCRDSpecVersion(sch, versionString(sch.Version()), sch.Version() == latest)
		if err != nil {
			return nil, err
		}
		resource.Spec.Versions = append(resource.Spec.Versions, ver)
		sch = sch.Successor()
	}
	contents, err := c.outputEncoder(resource)
	if err != nil {
		return nil, err
	}

	return codejen.NewFile(fmt.Sprintf("%s.%s.%s", meta.MachineName, decl.Def().Properties.CRD.Group, c.outputExtension), contents, c), nil
}

func schemaToCRDSpecVersion(sch thema.Schema, name string, stored bool) (k8s.CustomResourceDefinitionSpecVersion,
	error) {
	props, err := schemaToOpenAPIProperties(sch)
	if err != nil {
		return k8s.CustomResourceDefinitionSpecVersion{}, err
	}

	def := k8s.CustomResourceDefinitionSpecVersion{
		Name:    name,
		Served:  true,
		Storage: stored,
		Schema: map[string]any{
			"openAPIV3Schema": map[string]any{
				"properties": props,
				"required": []any{
					"spec",
				},
				"type": "object",
			},
		},
		Subresources: make(map[string]any),
	}

	for k := range props {
		if k != "spec" {
			def.Subresources[k] = struct{}{}
		}
	}

	return def, nil
}

// customResourceDefinition differs from k8s.CustomResourceDefinition in that it doesn't use the metav1
// TypeMeta and CommonMeta, as those do not contain YAML tags and get improperly serialized to YAML.
// Since we don't need to use it with the kubernetes go-client, we don't need the extra functionality attached.
//
//nolint:lll
type customResourceDefinition struct {
	Kind       string                           `json:"kind,omitempty" yaml:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	APIVersion string                           `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
	Metadata   customResourceDefinitionMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec       k8s.CustomResourceDefinitionSpec `json:"spec"`
}

type customResourceDefinitionMetadata struct {
	Name string `json:"name,omitempty" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	// TODO: other fields as necessary for codegen
}

type cueOpenAPIEncoded struct {
	Components cueOpenAPIEncodedComponents `json:"components"`
}

type cueOpenAPIEncodedComponents struct {
	Schemas map[string]any `json:"schemas"`
}

func schemaToOpenAPIProperties(sch thema.Schema) (map[string]any, error) {
	f, err := openapi.GenerateSchema(sch, &openapi.Config{
		Config: &cueopenapi.Config{
			ExpandReferences: true,
			NameFunc: func(val cue.Value, path cue.Path) string {
				return ""
			},
		},
	})
	if err != nil {
		return nil, err
	}

	str, err := cueyaml.Marshal(sch.Lineage().Runtime().Context().BuildFile(f))
	if err != nil {
		return nil, err
	}

	// Decode the bytes back into an object where we can trim the openAPI clutter out
	// and grab just the schema as a map[string]any (which is what k8s wants)
	back := cueOpenAPIEncoded{}
	err = goyaml.Unmarshal([]byte(str), &back)
	if err != nil {
		return nil, err
	}
	if len(back.Components.Schemas) != 1 {
		// There should only be one schema here...
		// TODO: this may change with subresources--but subresources should have defined names
		return nil, fmt.Errorf("version %s has multiple schemas", "")
	}
	var schemaProps map[string]any
	for k, v := range back.Components.Schemas {
		d, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("error generating openapi schema - generated schema has invalid type")
		}
		schemaProps, ok = d["properties"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("error generating openapi schema - %s has no properties", k)
		}
	}
	// Remove the "metadata" property, as metadata can't be extended in a CRD (the k8s.Client will handle how to encode/decode the metadata)
	delete(schemaProps, "metadata")

	// CRDs have a problem with openness and the "additionalProperties: {}", we need to _instead_ use "x-kubernetes-preserve-unknown-fields": true
	replaceAdditionalProperties(schemaProps)

	return schemaProps, nil
}

func replaceAdditionalProperties(props map[string]any) {
	for _, v := range props {
		cast, ok := v.(map[string]any)
		if !ok {
			return
		}
		if val, ok := cast["additionalProperties"]; ok {
			castVal, ok := val.(map[string]any)
			if !ok {
				return
			}
			if len(castVal) == 0 {
				delete(cast, "additionalProperties")
				cast["x-kubernetes-preserve-unknown-fields"] = true
			} else if innerProps, ok := castVal["properties"]; ok {
				castInnerProps, ok := innerProps.(map[string]any)
				if !ok {
					return
				}
				replaceAdditionalProperties(castInnerProps)
				castVal["properties"] = castInnerProps
				cast["additionalProperties"] = castVal
			}
		}
		if innerProps, ok := cast["properties"]; ok {
			castInnerProps, ok := innerProps.(map[string]any)
			if !ok {
				return
			}
			replaceAdditionalProperties(castInnerProps)
			cast["properties"] = castInnerProps
		}
	}
}
