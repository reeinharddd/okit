// Package classifier provides the Task Classifier for okit.
// It implements a hybrid model selection system for classifying user tasks.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package classifier

import (
	"context"
	"errors"
	"sort"
)

// modelSelector implements the ModelSelector interface.
type modelSelector struct {
	registry ProviderRegistry
	db      DBInterface
}

// NewModelSelector creates a new ModelSelector.
func NewModelSelector(registry ProviderRegistry, db DBInterface) ModelSelector {
	return &modelSelector{
		registry: registry,
		db:      db,
	}
}

// SelectModel selects the best model for classifying a task.
func (s *modelSelector) SelectModel(ctx context.Context, task Task) (Model, error) {
	// Get all available models from all providers.
	providers := s.registry.Providers()
	var models []Model
	for _, provider := range providers {
		providerModels, err := provider.Models(ctx)
		if err != nil {
			return Model{}, err
			}
		models = append(models, providerModels...)
	}

	if len(models) == 0 {
		return Model{}, errors.New("no models available")
	}

	// Sort models by priority:
	// 1. Free tier models (ascending cost)
	// 2. Non-free tier models (ascending cost)
	// 3. Latency (ascending)
	sort.Slice(models, func(i, j int) bool {
		if models[i].IsFreeTier != models[j].IsFreeTier {
			return models[i].IsFreeTier
			}
		if models[i].Cost != models[j].Cost {
			return models[i].Cost < models[j].Cost
			}
		return models[i].Latency < models[j].Latency
	})

	// Select the highest priority model.
	return models[0], nil
}
