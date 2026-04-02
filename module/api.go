package module

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type Module interface {
	Enabled() bool
	Description() (name, desc string)
	Dependencies() []string
	Init(ctx context.Context) error
}

type DefaultModuleWrapper struct {
	name, desc   string
	initFn       func(ctx context.Context) error
	dependencies []string
}

func NewModuleWrapper(
	name, desc string,
	initFn func(context.Context) error,
	dependencies ...string,
) *DefaultModuleWrapper {
	return &DefaultModuleWrapper{
		name:         strings.TrimSpace(name),
		desc:         strings.TrimSpace(desc),
		initFn:       initFn,
		dependencies: normalizeDependencies(dependencies),
	}
}

func (m *DefaultModuleWrapper) Enabled() bool {
	return true
}

func (m *DefaultModuleWrapper) Description() (name, desc string) {
	return m.name, m.desc
}

func (m *DefaultModuleWrapper) Dependencies() []string {
	return slices.Clone(m.dependencies)
}

func (m *DefaultModuleWrapper) Init(ctx context.Context) error {
	return m.initFn(ctx)
}

func Sort(modules ...Module) ([]Module, error) {
	type node struct {
		module     Module
		name       string
		index      int
		dependsOn  []string
		dependents []string
		inDegree   int
	}

	nodes := make([]*node, 0, len(modules))
	nodesByName := make(map[string]*node, len(modules))
	for index, candidate := range modules {
		if candidate == nil || !candidate.Enabled() {
			continue
		}

		name, _ := candidate.Description()
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("module at index %d has empty name", index)
		}
		if _, exists := nodesByName[name]; exists {
			return nil, fmt.Errorf("duplicate module %q", name)
		}

		node := &node{
			module:    candidate,
			name:      name,
			index:     index,
			dependsOn: normalizeDependencies(candidate.Dependencies()),
		}
		nodes = append(nodes, node)
		nodesByName[name] = node
	}

	for _, current := range nodes {
		for _, dependency := range current.dependsOn {
			if dependency == current.name {
				return nil, fmt.Errorf("module %q cannot depend on itself", current.name)
			}

			prerequisite, exists := nodesByName[dependency]
			if !exists {
				return nil, fmt.Errorf("module %q depends on unknown module %q", current.name, dependency)
			}

			current.inDegree++
			prerequisite.dependents = append(prerequisite.dependents, current.name)
		}
	}

	ready := make([]*node, 0, len(nodes))
	for _, current := range nodes {
		if current.inDegree == 0 {
			ready = append(ready, current)
		}
	}
	slices.SortFunc(ready, func(left, right *node) int {
		return left.index - right.index
	})

	ordered := make([]Module, 0, len(nodes))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		ordered = append(ordered, current.module)

		for _, dependentName := range current.dependents {
			dependent := nodesByName[dependentName]
			dependent.inDegree--
			if dependent.inDegree == 0 {
				ready = append(ready, dependent)
				slices.SortFunc(ready, func(left, right *node) int {
					return left.index - right.index
				})
			}
		}
	}

	if len(ordered) != len(nodes) {
		blocked := make([]string, 0, len(nodes)-len(ordered))
		for _, current := range nodes {
			if current.inDegree > 0 {
				blocked = append(blocked, current.name)
			}
		}
		slices.Sort(blocked)
		return nil, fmt.Errorf("module dependency cycle detected among: %s", strings.Join(blocked, ", "))
	}

	return ordered, nil
}

func Execute(ctx context.Context, modules ...Module) error {
	ordered, err := Sort(modules...)
	if err != nil {
		return err
	}

	for _, current := range ordered {
		name, _ := current.Description()
		if err := current.Init(ctx); err != nil {
			return fmt.Errorf("init module %q: %w", strings.TrimSpace(name), err)
		}
	}

	return nil
}

func normalizeDependencies(dependencies []string) []string {
	if len(dependencies) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(dependencies))
	seen := make(map[string]struct{}, len(dependencies))
	for _, dependency := range dependencies {
		dependency = strings.TrimSpace(dependency)
		if dependency == "" {
			continue
		}
		if _, exists := seen[dependency]; exists {
			continue
		}
		seen[dependency] = struct{}{}
		ordered = append(ordered, dependency)
	}

	return ordered
}

var _ Module = (*DefaultModuleWrapper)(nil)
