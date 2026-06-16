// Package classifier provides the Task Classifier for okit.
// It implements a hybrid model selection system for classifying user tasks.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package classifier

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sync"

	"github.com/reeinharddd/okit/internal/classifier/tokenizer"
	"github.com/reeinharddd/okit/pkg/onnx"
)

//go:embed embedded/*
var embeddedFS embed.FS

// EmbeddedProvider implements the Provider interface for embedded models.
type EmbeddedProvider struct {
	model     *onnx.Model
	tokenizer *tokenizer.Tokenizer
	mu        sync.Mutex
}

// NewEmbeddedProvider creates a new EmbeddedProvider.
func NewEmbeddedProvider() (*EmbeddedProvider, error) {
	// Load the ONNX model.
	modelData, err := embeddedFS.ReadFile("embedded/model.onnx")
	if err != nil {
		return nil, fmt.Errorf("read model.onnx: %w", err)
	}

	model, err := onnx.LoadModel(modelData)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	// Load the tokenizer.
	tokenizerData, err := embeddedFS.ReadFile("embedded/tokenizer.json")
	if err != nil {
		return nil, fmt.Errorf("read tokenizer.json: %w", err)
	}

	tok, err := tokenizer.NewTokenizer(tokenizerData)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	return &EmbeddedProvider{
		model:     model,
		tokenizer: tok,
	}, nil
}

// ID returns the unique identifier for the provider.
func (p *EmbeddedProvider) ID() string {
	return "embedded"
}

// Name returns the human-readable name of the provider.
func (p *EmbeddedProvider) Name() string {
	return "Embedded Provider"
}

// Models returns the list of models available from this provider.
func (p *EmbeddedProvider) Models(ctx context.Context) ([]Model, error) {
	return []Model{
		{
			ID:          "embedded-distilbert",
			Name:        "DistilBERT (Embedded)",
			Provider:     p.ID(),
			Latency:     100, // Estimated latency in milliseconds
			Cost:        0,   // No cost for embedded models
			IsFreeTier:  true,
		},
	}, nil
}

// Classify classifies a task using the embedded model.
func (p *EmbeddedProvider) Classify(ctx context.Context, task Task, model Model) (ClassificationResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if model.Provider != p.ID() {
		return ClassificationResult{}, errors.New("model not supported by this provider")
	}

	// Tokenize the input.
	tokens, err := p.tokenizer.Encode(task.Input)
	if err != nil {
		return ClassificationResult{}, fmt.Errorf("tokenize input: %w", err)
	}

	// Run inference.
	inputs := map[string][]float32{"input_ids": tokens}
	if _, err := p.model.Run(inputs); err != nil {
		return ClassificationResult{}, fmt.Errorf("inference: %w", err)
	}

	// Parse the output.
	// TODO: Implement output parsing logic.
	intent := "unknown"
	confidence := 0.0
	entities := make(map[string]string)

	return ClassificationResult{
		TaskID:     task.ID,
		ModelID:    model.ID,
		Intent:     intent,
		Confidence: confidence,
		Entities:   entities,
		Latency:    100, // Placeholder
	}, nil
}