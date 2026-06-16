// Package db provides database operations for okit.
//
// Copyright 2026 OpenCode Foundation
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"database/sql"
	"github.com/reeinharddd/okit/pkg/models"
)

// DBInterface defines the interface for database operations.
type DBInterface interface {
	// Provider operations
	UpsertProvider(p *models.Provider) error
	ListProviders() ([]models.Provider, error)
	GetProvider(id string) (*models.Provider, error)
	DeleteProvider(id string) error

	// Model operations
	UpsertModel(m *models.Model) error
	ListModels(opts ...ModelFilter) ([]models.Model, error)
	ListModelsByProvider(providerID string) ([]models.Model, error)
	GetModel(id string) (*models.Model, error)
	DeleteModel(id string) error

	// Command operations
	UpsertCommand(c *models.Command) error
	ListCommands() ([]models.Command, error)

	// MCP operations
	UpsertMCP(m *models.MCPServer) error
	ListMCPs() ([]models.MCPServer, error)

	// Skill operations
	UpsertSkill(s *models.Skill) error
	ListSkills() ([]models.Skill, error)

	// Source operations
	UpsertSourceItem(s *models.SourceItem) error
	ListSourceItems() ([]models.SourceItem, error)
	GetSourceItem(id string) (*models.SourceItem, error)
	DeleteSourceItem(id string) error

	// LSP operations
	UpsertLSPServer(l *models.LSPServer) error
	ListLSPServers() ([]models.LSPServer, error)
	GetLSPServer(id string) (*models.LSPServer, error)
	DeleteLSPServer(id string) error

	// Config fragment operations
	UpsertConfigFragment(f *models.ConfigFragment) error
	ListConfigFragments(limit int) ([]models.ConfigFragment, error)
	GetConfigFragment(id string) (*models.ConfigFragment, error)

	// Model profile operations
	UpsertModelProfile(p *models.ModelProfile) error
	ListModelProfiles() ([]models.ModelProfile, error)
	GetModelProfile(modelID string) (*models.ModelProfile, error)

	// Source registry operations
	UpsertSource(src *models.Source) error
	ListSources() ([]models.Source, error)

	// Agent operations
	UpsertAgent(a *models.Agent) error
	ListAgents() ([]models.Agent, error)

	// Routing operations
	InsertRoutingEvent(e *models.RoutingEvent) error
	ListRoutingRules() ([]models.RoutingRule, error)
	UpsertRoutingRule(r *models.RoutingRule) error
	GetBudget() (*models.BudgetConfig, error)

	// Preference operations
	SetPreference(key, value string) error
	ListPreferences() (map[string]string, error)

	// Raw query operations (for heal/migration)
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)

	// Small fast models for classification
	GetSmallFastModels(ctx context.Context) ([]models.Model, error)

	// DB path
	DBPath() string
}

// GetSmallFastModels retrieves small, fast models for classification.
func (d *DB) GetSmallFastModels(ctx context.Context) ([]models.Model, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, display_name, provider_id, latency_p50_ms, pricing_prompt, pricing_completion, tier
		FROM models
		WHERE latency_p50_ms < 500 AND (pricing_prompt + pricing_completion) <= 0.01
		ORDER BY 
			CASE WHEN tier = 'free' THEN 0 ELSE 1 END,
			(pricing_prompt + pricing_completion) ASC,
			latency_p50_ms ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Model
	for rows.Next() {
		var m models.Model
		var latency float64
		var promptCost, completionCost float64
		var tier string
		if err := rows.Scan(&m.ID, &m.DisplayName, &m.ProviderID, &latency, &promptCost, &completionCost, &tier); err != nil {
			return nil, err
		}
		m.LatencyP50Ms = latency
		m.PricingPrompt = promptCost
		m.PricingCompletion = completionCost
		m.Tier = tier
		result = append(result, m)
	}
	return result, nil
}

// MockDB is a mock implementation of DBInterface for testing.
type MockDB struct {
	GetSmallFastModelsFunc func(ctx context.Context) ([]models.Model, error)
	Providers              []models.Provider
	Models                 []models.Model
	Agents                 []models.Agent
	Commands               []models.Command
	MCPServers             []models.MCPServer
	Skills                 []models.Skill
	Items                  []models.SourceItem
	LSPServers             []models.LSPServer
	Fragments              []models.ConfigFragment
	Profiles               []models.ModelProfile
	Sources                []models.Source
	RoutingRules           []models.RoutingRule
	Preferences            map[string]string
}

func (m *MockDB) Query(query string, args ...any) (*sql.Rows, error) { return nil, nil }
func (m *MockDB) Exec(query string, args ...any) (sql.Result, error) { return nil, nil }
func (m *MockDB) DBPath() string                                     { return ":memory:" }

func (m *MockDB) UpsertProvider(p *models.Provider) error {
	if m.Providers == nil {
		m.Providers = make([]models.Provider, 0)
	}
	m.Providers = append(m.Providers, *p)
	return nil
}
func (m *MockDB) ListProviders() ([]models.Provider, error)       { return m.Providers, nil }
func (m *MockDB) GetProvider(id string) (*models.Provider, error) { return nil, nil }
func (m *MockDB) DeleteProvider(id string) error                  { return nil }

func (m *MockDB) UpsertModel(model *models.Model) error {
	if m.Models == nil {
		m.Models = make([]models.Model, 0)
	}
	m.Models = append(m.Models, *model)
	return nil
}
func (m *MockDB) ListModels(opts ...ModelFilter) ([]models.Model, error) { return m.Models, nil }
func (m *MockDB) ListModelsByProvider(providerID string) ([]models.Model, error) { return m.Models, nil }
func (m *MockDB) GetModel(id string) (*models.Model, error)              { return nil, nil }
func (m *MockDB) DeleteModel(id string) error                            { return nil }

func (m *MockDB) UpsertCommand(c *models.Command) error {
	if m.Commands == nil {
		m.Commands = make([]models.Command, 0)
	}
	m.Commands = append(m.Commands, *c)
	return nil
}
func (m *MockDB) ListCommands() ([]models.Command, error) { return m.Commands, nil }

func (m *MockDB) UpsertMCP(mcp *models.MCPServer) error {
	if m.MCPServers == nil {
		m.MCPServers = make([]models.MCPServer, 0)
	}
	m.MCPServers = append(m.MCPServers, *mcp)
	return nil
}
func (m *MockDB) ListMCPs() ([]models.MCPServer, error) { return m.MCPServers, nil }

func (m *MockDB) UpsertSkill(s *models.Skill) error {
	if m.Skills == nil {
		m.Skills = make([]models.Skill, 0)
	}
	m.Skills = append(m.Skills, *s)
	return nil
}
func (m *MockDB) ListSkills() ([]models.Skill, error) { return m.Skills, nil }

func (m *MockDB) UpsertSourceItem(s *models.SourceItem) error {
	if m.Items == nil {
		m.Items = make([]models.SourceItem, 0)
	}
	m.Items = append(m.Items, *s)
	return nil
}
func (m *MockDB) ListSourceItems() ([]models.SourceItem, error) { return m.Items, nil }
func (m *MockDB) GetSourceItem(id string) (*models.SourceItem, error) { return nil, nil }
func (m *MockDB) DeleteSourceItem(id string) error                     { return nil }

func (m *MockDB) UpsertLSPServer(l *models.LSPServer) error {
	if m.LSPServers == nil {
		m.LSPServers = make([]models.LSPServer, 0)
	}
	m.LSPServers = append(m.LSPServers, *l)
	return nil
}
func (m *MockDB) ListLSPServers() ([]models.LSPServer, error) { return m.LSPServers, nil }
func (m *MockDB) GetLSPServer(id string) (*models.LSPServer, error) { return nil, nil }
func (m *MockDB) DeleteLSPServer(id string) error                     { return nil }

func (m *MockDB) UpsertConfigFragment(f *models.ConfigFragment) error {
	if m.Fragments == nil {
		m.Fragments = make([]models.ConfigFragment, 0)
	}
	m.Fragments = append(m.Fragments, *f)
	return nil
}
func (m *MockDB) ListConfigFragments(limit int) ([]models.ConfigFragment, error) { return m.Fragments, nil }
func (m *MockDB) GetConfigFragment(id string) (*models.ConfigFragment, error)    { return nil, nil }

func (m *MockDB) UpsertModelProfile(p *models.ModelProfile) error {
	if m.Profiles == nil {
		m.Profiles = make([]models.ModelProfile, 0)
	}
	m.Profiles = append(m.Profiles, *p)
	return nil
}
func (m *MockDB) ListModelProfiles() ([]models.ModelProfile, error) { return m.Profiles, nil }
func (m *MockDB) GetModelProfile(modelID string) (*models.ModelProfile, error) { return nil, nil }

func (m *MockDB) UpsertSource(src *models.Source) error {
	if m.Sources == nil {
		m.Sources = make([]models.Source, 0)
	}
	m.Sources = append(m.Sources, *src)
	return nil
}
func (m *MockDB) ListSources() ([]models.Source, error) { return m.Sources, nil }

func (m *MockDB) UpsertAgent(a *models.Agent) error {
	if m.Agents == nil {
		m.Agents = make([]models.Agent, 0)
	}
	m.Agents = append(m.Agents, *a)
	return nil
}
func (m *MockDB) ListAgents() ([]models.Agent, error) { return m.Agents, nil }

func (m *MockDB) InsertRoutingEvent(e *models.RoutingEvent) error    { return nil }
func (m *MockDB) ListRoutingRules() ([]models.RoutingRule, error)    { return m.RoutingRules, nil }
func (m *MockDB) UpsertRoutingRule(r *models.RoutingRule) error {
	if m.RoutingRules == nil {
		m.RoutingRules = make([]models.RoutingRule, 0)
	}
	m.RoutingRules = append(m.RoutingRules, *r)
	return nil
}
func (m *MockDB) GetBudget() (*models.BudgetConfig, error) {
	return &models.BudgetConfig{ID: "default", DailyGlobalUSD: 0.50, PreferredTier: "free_only"}, nil
}

func (m *MockDB) SetPreference(key, value string) error {
	if m.Preferences == nil {
		m.Preferences = make(map[string]string)
	}
	m.Preferences[key] = value
	return nil
}
func (m *MockDB) ListPreferences() (map[string]string, error) { return m.Preferences, nil }

func (m *MockDB) GetSmallFastModels(ctx context.Context) ([]models.Model, error) {
	if m.GetSmallFastModelsFunc != nil {
		return m.GetSmallFastModelsFunc(ctx)
	}
	return nil, nil
}