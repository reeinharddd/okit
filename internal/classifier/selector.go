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

type modelSelector struct {
	registry ProviderRegistry
}

func NewModelSelector(registry ProviderRegistry) ModelSelector {
	return &modelSelector{registry: registry}
}

func (s *modelSelector) SelectModel(ctx context.Context, task Task) (Model, error) {
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

	sort.Slice(models, func(i, j int) bool {
		if models[i].IsFreeTier != models[j].IsFreeTier {
			return models[i].IsFreeTier
		}
		if models[i].Cost != models[j].Cost {
			return models[i].Cost < models[j].Cost
		}
		return models[i].Latency < models[j].Latency
	})

	return models[0], nil
}
