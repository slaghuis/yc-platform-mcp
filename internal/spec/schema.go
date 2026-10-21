package spec

import (
	"strings"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

const (
	schemaRefPrefix = "#/components/schemas/"
	maxSchemaDepth  = 12
)

func buildSchema(service, name string, proxy *base.SchemaProxy) SchemaDefinition {
	stack := map[string]bool{}
	if name != "" {
		stack[name] = true
	}
	def := walkSchema(proxy, stack, 0)
	def.Service = service
	def.Name = name
	return def
}

func walkSchema(proxy *base.SchemaProxy, stack map[string]bool, depth int) SchemaDefinition {
	if proxy == nil {
		return SchemaDefinition{}
	}
	ref := proxy.GetReference()
	refName := schemaRefName(ref)
	if refName != "" && stack[refName] {
		return SchemaDefinition{Ref: ref}
	}
	if depth > maxSchemaDepth {
		if ref != "" {
			return SchemaDefinition{Ref: ref}
		}
		return SchemaDefinition{Description: "schema omitted: maximum depth reached"}
	}
	if refName != "" {
		stack[refName] = true
		defer delete(stack, refName)
	}

	resolved := proxy.Schema()
	if resolved == nil {
		return SchemaDefinition{Ref: ref}
	}
	def := schemaFromResolved(resolved, ref, stack, depth)
	return def
}

func schemaFromResolved(resolved *base.Schema, ref string, stack map[string]bool, depth int) SchemaDefinition {
	def := SchemaDefinition{
		Description: strings.TrimSpace(resolved.Description),
		Type:        strings.Join(resolved.Type, ", "),
		Format:      resolved.Format,
		Required:    copyStrings(resolved.Required),
		Enum:        enumValues(resolved.Enum),
		Ref:         ref,
	}
	if resolved.Properties != nil {
		for name, prop := range resolved.Properties.FromOldest() {
			def.Properties = append(def.Properties, SchemaProperty{
				Name:   name,
				Schema: walkSchema(prop, stack, depth+1),
			})
		}
	}
	if resolved.Items != nil && resolved.Items.IsA() && resolved.Items.A != nil {
		item := walkSchema(resolved.Items.A, stack, depth+1)
		def.Items = &item
	}
	if resolved.AdditionalProperties != nil {
		switch {
		case resolved.AdditionalProperties.IsA() && resolved.AdditionalProperties.A != nil:
			extra := walkSchema(resolved.AdditionalProperties.A, stack, depth+1)
			def.AdditionalProperties = &extra
		case resolved.AdditionalProperties.IsB():
			allowed := resolved.AdditionalProperties.B
			def.AdditionalPropertiesAllowed = &allowed
		}
	}
	def.AllOf = walkComposition(resolved.AllOf, stack, depth)
	def.OneOf = walkComposition(resolved.OneOf, stack, depth)
	def.AnyOf = walkComposition(resolved.AnyOf, stack, depth)
	return def
}

func walkComposition(parts []*base.SchemaProxy, stack map[string]bool, depth int) []SchemaDefinition {
	if len(parts) == 0 {
		return nil
	}
	out := make([]SchemaDefinition, 0, len(parts))
	for _, part := range parts {
		out = append(out, walkSchema(part, stack, depth+1))
	}
	return out
}

func schemaRefName(ref string) string {
	if !strings.HasPrefix(ref, schemaRefPrefix) {
		return ""
	}
	name := strings.TrimPrefix(ref, schemaRefPrefix)
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name = name[:i]
	}
	return name
}

func enumValues(nodes []*yaml.Node) []string {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if value := enumValue(node); value != "" || (node != nil && node.Tag == "!!null") {
			out = append(out, value)
		}
	}
	return out
}

func enumValue(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		return enumValue(node.Alias)
	}
	return node.Value
}

func copyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func eachPair[K comparable, V any](m *orderedmap.Map[K, V], fn func(K, V)) {
	if m == nil {
		return
	}
	for key, value := range m.FromOldest() {
		fn(key, value)
	}
}
