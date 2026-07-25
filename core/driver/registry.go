// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is an explicit collection of Drivers, keyed by model name.
// There is deliberately NO package-level default registry and NO init()
// self-registration anywhere in this project: whoever assembles an
// application (CLI, GUI) constructs a Registry and registers exactly the
// drivers it means to offer, so the set of radios a binary supports is
// always visible at its composition root, never assembled invisibly by
// import side effects.
//
// A Registry is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{drivers: make(map[string]Driver)}
}

// Register adds d, keyed by d.Model(). It rejects (with an error, never a
// panic): a nil Driver; a duplicate model name; a d whose Model()
// disagrees with its Capabilities().Model (the two must be the same
// string — a mismatch means lookups and capability data would name
// different radios); and a d whose Capabilities() fail
// spec.Capabilities.Validate — the registry is where the "every driver's
// capability data is structurally valid" invariant is enforced centrally,
// so generic code reading Capabilities from a registered driver never
// needs to re-validate.
func (r *Registry) Register(d Driver) error {
	if d == nil {
		return fmt.Errorf("driver: Register: driver must not be nil")
	}
	model := d.Model()
	caps := d.Capabilities()
	if model != caps.Model {
		return fmt.Errorf("driver: Register: Model() %q disagrees with Capabilities().Model %q", model, caps.Model)
	}
	if err := caps.Validate(); err != nil {
		return fmt.Errorf("driver: Register %q: invalid capabilities: %w", model, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.drivers[model]; exists {
		return fmt.Errorf("driver: Register: model %q is already registered", model)
	}
	r.drivers[model] = d
	return nil
}

// Get returns the Driver registered for model and true, or nil and false
// if no driver is registered under that name.
func (r *Registry) Get(model string) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drivers[model]
	return d, ok
}

// Models returns every registered model name, sorted, so callers (a CLI
// listing supported radios, a GUI picker) get deterministic output.
func (r *Registry) Models() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	models := make([]string, 0, len(r.drivers))
	for m := range r.drivers {
		models = append(models, m)
	}
	sort.Strings(models)
	return models
}
