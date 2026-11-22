package patterns

import "sort"

// Registry holds the known patterns by type.
type Registry struct {
	byType map[string]Pattern
}

// NewRegistry registers patterns. A later pattern with the same type
// replaces an earlier one.
func NewRegistry(ps ...Pattern) *Registry {
	r := &Registry{byType: make(map[string]Pattern, len(ps))}
	for _, p := range ps {
		r.byType[p.Meta().Type] = p
	}
	return r
}

// Get returns the pattern of a type.
func (r *Registry) Get(typ string) (Pattern, bool) {
	p, ok := r.byType[typ]
	return p, ok
}

// All returns every pattern's metadata, sorted by type.
func (r *Registry) All() []Meta {
	out := make([]Meta, 0, len(r.byType))
	for _, p := range r.byType {
		out = append(out, p.Meta())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// Default returns a registry of every V1 pattern and filter.
func Default() *Registry {
	return NewRegistry(NYearHigh{}, VolumeSurge{}, RelativeStrength{}, NotExtended{}, LevelAge{})
}
