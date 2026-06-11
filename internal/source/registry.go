package source

import (
	"fmt"

	"github.com/wu/agentsstore/internal/config"
)

type Factory func(spec config.Source) Source

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}}
}

func (r *Registry) Register(typeName string, f Factory) {
	r.factories[typeName] = f
}

func (r *Registry) Build(spec config.Source) (Source, error) {
	f, ok := r.factories[spec.Type]
	if !ok {
		return nil, fmt.Errorf("unknown source type %q", spec.Type)
	}
	return f(spec), nil
}

func (r *Registry) BuildAll(cfg *config.Config) ([]Source, error) {
	out := make([]Source, 0, len(cfg.Sources))
	for _, s := range cfg.Sources {
		src, err := r.Build(s)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}
