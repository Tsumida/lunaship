package module

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	t.Run("flow: execute orders modules by declared dependencies", func(t *testing.T) {
		// Description: modules are registered out of order while later modules depend on earlier bootstrap stages.
		// Procedure: Execute the graph with recorder callbacks and capture the observed initialization order.
		// Expectation: initialization should follow dependency order rather than registration order.
		order := make([]string, 0, 3)
		modules := []Module{
			NewModuleWrapper("http", "init http", func(ctx context.Context) error {
				order = append(order, "http")
				return nil
			}, "mysql"),
			NewModuleWrapper("config", "load config", func(ctx context.Context) error {
				order = append(order, "config")
				return nil
			}),
			NewModuleWrapper("mysql", "init mysql", func(ctx context.Context) error {
				order = append(order, "mysql")
				return nil
			}, "config"),
		}

		err := Execute(context.Background(), modules...)

		assert.NoError(t, err, "dependency-ordered execution should succeed")
		assert.Equal(t, []string{"config", "mysql", "http"}, order, "execution order should satisfy dependencies")
	})

	t.Run("flow: execute wraps the failing module name", func(t *testing.T) {
		// Description: a module callback fails after dependency resolution succeeds.
		// Procedure: Execute a single module that returns a sentinel error.
		// Expectation: the returned error should include the module name and preserve the original cause.
		sentinel := errors.New("boom")

		err := Execute(context.Background(), NewModuleWrapper("redis", "init redis", func(ctx context.Context) error {
			return sentinel
		}))

		assert.Error(t, err, "module execution should surface callback failures")
		assert.ErrorIs(t, err, sentinel, "execution should preserve the original failure")
		assert.Contains(t, err.Error(), `init module "redis"`, "error should identify the failing module")
	})
}

func TestSort(t *testing.T) {
	t.Run("flow: sort rejects unknown dependencies", func(t *testing.T) {
		// Description: a module refers to a prerequisite that is not registered in the graph.
		// Procedure: Sort a graph containing a dangling dependency.
		// Expectation: sorting should fail fast with a clear dependency error.
		_, err := Sort(
			NewModuleWrapper("http", "init http", func(ctx context.Context) error { return nil }, "config"),
		)

		assert.Error(t, err, "sorting should fail when a dependency is missing")
		assert.Contains(t, err.Error(), `depends on unknown module "config"`, "error should name the missing dependency")
	})

	t.Run("flow: sort rejects dependency cycles", func(t *testing.T) {
		// Description: two modules depend on each other and therefore cannot be initialized.
		// Procedure: Sort a cyclic graph.
		// Expectation: sorting should fail with a cycle report instead of deadlocking or producing partial order.
		_, err := Sort(
			NewModuleWrapper("config", "load config", func(ctx context.Context) error { return nil }, "log"),
			NewModuleWrapper("log", "init log", func(ctx context.Context) error { return nil }, "config"),
		)

		assert.Error(t, err, "sorting should fail for cyclic graphs")
		assert.Contains(t, err.Error(), "module dependency cycle detected", "error should explain the graph failure")
	})
}
