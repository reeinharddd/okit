// Package classifier provides the Task Classifier for okit.
// It implements a hybrid model selection system for classifying user tasks.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package classifier

import (
	"context"
)

// Classifier defines the interface for classifying user tasks.
type Classifier interface {
	// Classify classifies a user task and returns the result.
	Classify(ctx context.Context, task Task) (ClassificationResult, error)
}

// ModelSelector defines the interface for selecting a model for classification.
type ModelSelector interface {
	// SelectModel selects the best model for classifying a task.
	SelectModel(ctx context.Context, task Task) (Model, error)
}

// Cache defines the interface for caching classification results.
type Cache interface {
	// Get retrieves a cached classification result.
	Get(ctx context.Context, key CacheKey) (ClassificationResult, bool)
	// Set stores a classification result in the cache.
	Set(ctx context.Context, key CacheKey, result ClassificationResult)
	// Delete removes a classification result from the cache.
	Delete(ctx context.Context, key CacheKey)
}

// ProviderRegistry defines the interface for managing model providers.
type ProviderRegistry interface {
	// Register registers a provider with the registry.
	Register(provider Provider) error
	// Providers returns the list of registered providers.
	Providers() []Provider
	// Provider returns the provider with the given ID.
	Provider(id string) (Provider, bool)
}