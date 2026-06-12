// Package mcp provides the MCP server for okit.
// It exposes tools and resources for AI agents to interact with okit.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/opencodeco/okit/internal/classifier"
	"github.com/opencodeco/okit/internal/db"
	"github.com/opencodeco/okit/pkg/models"
)

// Server represents an MCP server.
type Server struct {
	classifier classifier.Classifier
	db        db.DBInterface
	mu        sync.Mutex
	handlers  map[string]ToolHandler
}

// ToolHandler defines the interface for MCP tool handlers.
type ToolHandler func(ctx context.Context, req ToolRequest) (ToolResponse, error)

// ToolRequest represents a request to an MCP tool.
type ToolRequest struct {
	ToolID    string          `json:"tool_id"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResponse represents a response from an MCP tool.
type ToolResponse struct {
	Output interface{} `json:"output"`
	Error  string      `json:"error,omitempty"`
}

// NewServer creates a new MCP server.
func NewServer(classifier classifier.Classifier, db db.DBInterface) *Server {
	s := &Server{
		classifier: classifier,
		db:        db,
		handlers:  make(map[string]ToolHandler),
	}
	s.registerTools()
	return s
}

// registerTools registers the MCP tools.
func (s *Server) registerTools() {
	s.handlers["classify_task"] = s.handleClassifyTask
}

// Start starts the MCP server on the given address.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/tools", s.handleTools)
	mux.HandleFunc("/execute", s.handleExecute)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server.ListenAndServe()
}

// handleTools handles requests to list available tools.
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := []models.Tool{
		{
			ID:          "classify_task",
			Description: "Classify a user task into an intent and extract entities.",
			Parameters: map[string]models.ToolParameter{
				"task": {
					Type:        "object",
					Description: "The task to classify.",
					Properties: map[string]models.ToolParameter{
						"input":     {Type: "string", Description: "The raw user input to classify."},
						"session_id": {Type: "string", Description: "The ID of the current session."},
					},
					Required: []string{"input", "session_id"},
				},
			},
		},
	}

	_ = json.NewEncoder(w).Encode(tools)
}

// handleExecute handles requests to execute an MCP tool.
func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	handler, ok := s.handlers[req.ToolID]
	if !ok {
		http.Error(w, fmt.Sprintf("tool not found: %s", req.ToolID), http.StatusNotFound)
		return
	}

	resp, err := handler(r.Context(), req)
	if err != nil {
		resp.Error = err.Error()
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// handleClassifyTask handles requests to classify a task.
func (s *Server) handleClassifyTask(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	type Args struct {
		Input     string `json:"input"`
		SessionID string `json:"session_id"`
	}

	var args Args
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return ToolResponse{}, fmt.Errorf("unmarshal arguments: %w", err)
	}

	task := classifier.Task{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Input:     args.Input,
		SessionID: args.SessionID,
		CreatedAt: time.Now(),
	}

	result, err := s.classifier.Classify(ctx, task)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("classify task: %w", err)
	}

	return ToolResponse{Output: result}, nil
}