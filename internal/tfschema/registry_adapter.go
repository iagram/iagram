package tfschema

// Resource implements catalog.ResourceRegistry: settings-schema pieces for a
// resource type.
func (r *Registry) ResourceSchema(tfType string) (props map[string]any, required []string, outputs []string, ok bool) {
	b, _, ok := r.Resource(tfType)
	if !ok {
		return nil, nil, nil, false
	}
	props, required, outputs = b.JSONSchema()
	return props, required, outputs, true
}

// Types lists the resource types of a provider local name (empty when unknown).
func (r *Registry) Types(local string) []string {
	p, err := r.Provider(local)
	if err != nil {
		return nil
	}
	return p.Types()
}

// Catalog adapts the registry to the catalog's interface (method name clash:
// Resource returns the block here, the catalog wants schema pieces).
type Catalog struct{ *Registry }

// Resource satisfies catalog.ResourceRegistry.
func (c Catalog) Resource(tfType string) (map[string]any, []string, []string, bool) {
	return c.ResourceSchema(tfType)
}
