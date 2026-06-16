// Package classifier provides the Task Classifier for okit.
// It implements a hybrid model selection system for classifying user tasks.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package classifier

import (
	"sync"
)

// providerRegistry implements the ProviderRegistry interface.
type providerRegistry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewProviderRegistry creates a new ProviderRegistry.
func NewProviderRegistry() ProviderRegistry {
	return &providerRegistry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider with the registry.
func (r *providerRegistry) Register(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[provider.ID()] = provider
	return nil
}

// Providers returns the list of registered providers.
func (r *providerRegistry) Providers() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// Provider returns the provider with the given ID.
func (r *providerRegistry) Provider(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[id]
	return provider, ok
}

// DefaultProviders will be re-implemented when the adapter between
// db models and classifier models is needed. Currently deferred.
// TODO: implement DefaultProviders with Model type conversion