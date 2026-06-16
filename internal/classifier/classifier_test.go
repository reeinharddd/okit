// Package classifier provides the Task Classifier for okit.
// It implements a hybrid model selection system for classifying user tasks.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package classifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/reeinharddd/okit/internal/classifier"
	"github.com/stretchr/testify/assert"
)

// MockProvider is a mock implementation of the Provider interface.
type MockProvider struct {
	id      string
	name    string
	models  []classifier.Model
	classify func(ctx context.Context, task classifier.Task, model classifier.Model) (classifier.ClassificationResult, error)
}

func (m *MockProvider) ID() string {
	return m.id
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Models(ctx context.Context) ([]classifier.Model, error) {
	return m.models, nil
}

func (m *MockProvider) Classify(ctx context.Context, task classifier.Task, model classifier.Model) (classifier.ClassificationResult, error) {
	return m.classify(ctx, task, model)
}

// TestProviderRegistry tests the ProviderRegistry implementation.
func TestProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Register and Retrieve Providers", func(t *testing.T) {
		t.Parallel()
		registry := classifier.NewProviderRegistry()
		provider := &MockProvider{id: "mock", name: "Mock Provider"}

		// Register the provider.
		err := registry.Register(provider)
		assert.NoError(t, err)

		// Retrieve the provider.
		retrieved, ok := registry.Provider("mock")
		assert.True(t, ok)
		assert.Equal(t, "mock", retrieved.ID())
		assert.Equal(t, "Mock Provider", retrieved.Name())
	})

	t.Run("Providers List", func(t *testing.T) {
		t.Parallel()
		registry := classifier.NewProviderRegistry()
		provider1 := &MockProvider{id: "mock1", name: "Mock Provider 1"}
		provider2 := &MockProvider{id: "mock2", name: "Mock Provider 2"}

		// Register providers.
		_ = registry.Register(provider1)
		_ = registry.Register(provider2)

		// Retrieve the list of providers.
		providers := registry.Providers()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, provider1)
		assert.Contains(t, providers, provider2)
	})
}

// TestModelSelector tests the ModelSelector implementation.
func TestModelSelector(t *testing.T) {
	t.Parallel()

	t.Run("Select Free Tier Model", func(t *testing.T) {
		t.Parallel()
		registry := classifier.NewProviderRegistry()
		provider := &MockProvider{
			id:   "mock",
			name: "Mock Provider",
			models: []classifier.Model{
				{ID: "free1", Name: "Free Model 1", Provider: "mock", Latency: 100, Cost: 0, IsFreeTier: true},
				{ID: "paid1", Name: "Paid Model 1", Provider: "mock", Latency: 50, Cost: 1000, IsFreeTier: false},
				{ID: "free2", Name: "Free Model 2", Provider: "mock", Latency: 200, Cost: 0, IsFreeTier: true},
				},
			}
		_ = registry.Register(provider)

		// Create the ModelSelector.
		selector := classifier.NewModelSelector(registry)

		// Select the best model.
		task := classifier.Task{ID: "task1", Input: "test input", SessionID: "session1", CreatedAt: time.Now()}
		model, err := selector.SelectModel(context.Background(), task)
		assert.NoError(t, err)
		assert.Equal(t, "free1", model.ID) // Lowest latency free tier model.
	})

	t.Run("Select Paid Model if No Free Tier", func(t *testing.T) {
		t.Parallel()
		registry := classifier.NewProviderRegistry()
		provider := &MockProvider{
			id:   "mock",
			name: "Mock Provider",
			models: []classifier.Model{
				{ID: "paid1", Name: "Paid Model 1", Provider: "mock", Latency: 50, Cost: 1000, IsFreeTier: false},
				{ID: "paid2", Name: "Paid Model 2", Provider: "mock", Latency: 100, Cost: 2000, IsFreeTier: false},
				},
			}
		_ = registry.Register(provider)

		// Create the ModelSelector.
		selector := classifier.NewModelSelector(registry)

		// Select the best model.
		task := classifier.Task{ID: "task1", Input: "test input", SessionID: "session1", CreatedAt: time.Now()}
		model, err := selector.SelectModel(context.Background(), task)
		assert.NoError(t, err)
		assert.Equal(t, "paid1", model.ID) // Lowest cost paid model.
	})

	t.Run("No Models Available", func(t *testing.T) {
		t.Parallel()
		registry := classifier.NewProviderRegistry()
		provider := &MockProvider{
			id:     "mock",
			name:   "Mock Provider",
			models: []classifier.Model{},
			}
		_ = registry.Register(provider)

		// Create the ModelSelector.
		selector := classifier.NewModelSelector(registry)

		// Select the best model.
		task := classifier.Task{ID: "task1", Input: "test input", SessionID: "session1", CreatedAt: time.Now()}
		_, err := selector.SelectModel(context.Background(), task)
		assert.Error(t, err)
		assert.Equal(t, "no models available", err.Error())
	})
}