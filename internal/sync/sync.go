package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/internal/util"
	"github.com/reeinharddd/okit/pkg/models"
)

const metaPref = "config/"

type Service struct {
	db db.DBInterface
}

type Diff struct {
	AddedProviders   []string
	RemovedProviders []string
	AddedModels      []string
	RemovedModels    []string
	AddedAgents      []string
	RemovedAgents    []string
	AddedCommands    []string
	AddedMCPs        []string
}

func New(database db.DBInterface) *Service {
	return &Service{db: database}
}

func (s *Service) ImportFromOpenCodeConfig(configPath string) (*Diff, error) {
	diff := &Diff{}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cleaned := util.StripJSONC(data)
	var cfg map[string]interface{}
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	existing, _ := s.db.ListProviders()
	existingMap := make(map[string]bool)
	for _, p := range existing {
		existingMap[p.ID] = true
	}

	if provSection, ok := cfg["provider"].(map[string]interface{}); ok {
		for provID, provVal := range provSection {
			if provCfg, ok := provVal.(map[string]interface{}); ok {
				provName, _ := provCfg["name"].(string)
				if provName == "" {
					provName = provID
				}
				provBaseURL := ""
				if opts, ok := provCfg["options"].(map[string]interface{}); ok {
					if bu, _ := opts["baseURL"].(string); bu != "" {
						provBaseURL = bu
					}
				}
				upserted := !existingMap[provID]
				p := &models.Provider{
					ID:      provID,
					Name:    provName,
					BaseURL: provBaseURL,
					Source:  "opencode",
					Status:  "active",
				}
				// Preserve existing KeyEnv — don't wipe what discover set
				if existing, err := s.db.GetProvider(provID); err == nil && existing.KeyEnv != "" {
					p.KeyEnv = existing.KeyEnv
					p.CatalogURL = existing.CatalogURL
				}
				_ = s.db.UpsertProvider(p)
				if upserted {
					diff.AddedProviders = append(diff.AddedProviders, provID)
				}

				// Store npm and options as preferences
				if npm, _ := provCfg["npm"].(string); npm != "" && npm != "null" && npm != "NULL" {
					_ = s.db.SetPreference("config/provider_npm_"+provID, npm)
				}
				if opts, ok := provCfg["options"].(map[string]interface{}); ok && len(opts) > 0 {
					b, _ := json.Marshal(opts)
					_ = s.db.SetPreference("config/provider_options_"+provID, string(b))
				}

				importModels := func(names []string) {
					for _, name := range names {
						modelID := provID + "/" + name
						if err := s.db.UpsertModel(&models.Model{
							ID:          modelID,
							ProviderID:  provID,
							DisplayName: name,
							Source:      "opencode",
							Status:      "untested",
						}); err == nil {
							diff.AddedModels = append(diff.AddedModels, modelID)
						}
						_ = s.db.UpsertModelProfile(&models.ModelProfile{
							ModelID: modelID,
						})
					}
				}
				if whitelist, ok := provCfg["whitelist"].([]interface{}); ok {
					var names []string
					for _, w := range whitelist {
						if name, ok := w.(string); ok {
							names = append(names, name)
						}
					}
					importModels(names)
				}
				if modelsSection, ok := provCfg["models"].(map[string]interface{}); ok {
					var names []string
					for modelName := range modelsSection {
						names = append(names, modelName)
					}
					importModels(names)
				}
			}
		}
	}

	if agentSection, ok := cfg["agent"].(map[string]interface{}); ok {
		for agentID, agentVal := range agentSection {
			if agentMap, ok := agentVal.(map[string]interface{}); ok {
				model, _ := agentMap["model"].(string)
				desc, _ := agentMap["description"].(string)
				mode, _ := agentMap["mode"].(string)
				t, _ := agentMap["temperature"].(float64)
				color, _ := agentMap["color"].(string)

				_ = s.db.UpsertAgent(&models.Agent{
					ID:             agentID,
					Description:    desc,
					CurrentModelID: model,
					Mode:           mode,
					Temperature:    t,
					Color:          color,
					Source:         "opencode",
					Status:         "active",
				})
				diff.AddedAgents = append(diff.AddedAgents, agentID)
			}
		}
	}

	if cmdSection, ok := cfg["command"].(map[string]interface{}); ok {
		for cmdID, cmdVal := range cmdSection {
			if cmdMap, ok := cmdVal.(map[string]interface{}); ok {
				tpl, _ := cmdMap["template"].(string)
				desc, _ := cmdMap["description"].(string)

				_ = s.db.UpsertCommand(&models.Command{
					ID:          cmdID,
					Template:    tpl,
					Description: desc,
					Source:      "opencode",
					Status:      "active",
				})
				diff.AddedCommands = append(diff.AddedCommands, cmdID)
			}
		}
	}

	if mcpSection, ok := cfg["mcp"].(map[string]interface{}); ok {
		for id, val := range mcpSection {
			entry, _ := val.(map[string]interface{})
			m := models.MCPServer{ID: id, Source: "opencode"}
			if t, _ := entry["type"].(string); t != "" {
				m.Type = t
			}
			if cmd, _ := entry["command"].([]interface{}); len(cmd) > 0 {
				b, _ := json.Marshal(cmd)
				m.Command = string(b)
			}
			if u, _ := entry["url"].(string); u != "" {
				m.URL = u
			}
			if en, _ := entry["enabled"].(bool); en {
				m.Enabled = true
			}
			if env, _ := entry["environment"].(map[string]interface{}); len(env) > 0 {
				b, _ := json.Marshal(env)
				m.EnvVars = string(b)
			}
			if to, _ := entry["timeout"].(float64); to > 0 {
				m.Timeout = int(to)
			}
			_ = s.db.UpsertMCP(&m)
			diff.AddedMCPs = append(diff.AddedMCPs, id)
		}
	}

	if lspBool, isBool := cfg["lsp"].(bool); isBool {
		_ = s.db.SetPreference(metaPref+"lsp", fmt.Sprintf("%t", lspBool))
	} else if lspObj, isObj := cfg["lsp"].(map[string]interface{}); isObj {
		_ = s.db.SetPreference(metaPref+"lsp", "object")
		for id, val := range lspObj {
			entry, _ := val.(map[string]interface{})
			l := models.LSPServer{ID: id}
			if cmd, _ := entry["command"].([]interface{}); len(cmd) > 0 {
				b, _ := json.Marshal(cmd)
				l.Command = string(b)
			}
			if ext, _ := entry["extensions"].([]interface{}); len(ext) > 0 {
				b, _ := json.Marshal(ext)
				l.Extensions = string(b)
			}
			if env, _ := entry["env"].(map[string]interface{}); len(env) > 0 {
				b, _ := json.Marshal(env)
				l.Env = string(b)
			}
			if init, _ := entry["initialization"].(string); init != "" {
				l.Initialization = init
			}
			if dis, _ := entry["disabled"].(bool); dis {
				l.Disabled = true
			}
			_ = s.db.UpsertLSPServer(&l)
		}
	}

	setJSONPref := func(key string, val interface{}) {
		b, _ := json.Marshal(val)
		_ = s.db.SetPreference(key, string(b))
	}
	for _, key := range []string{"autoupdate", "disabled_providers", "model", "small_model", "share", "plugin"} {
		if v, exists := cfg[key]; exists {
			setJSONPref(metaPref+key, v)
		}
	}
	if skills, ok := cfg["skills"].(map[string]interface{}); ok {
		for sk, sv := range skills {
			setJSONPref(metaPref+"skills_"+sk, sv)
		}
	}
	if comp, ok := cfg["compaction"].(map[string]interface{}); ok {
		for ck, cv := range comp {
			setJSONPref(metaPref+"compaction_"+ck, cv)
		}
	}

	return diff, nil
}

func (s *Service) ExportToOpenCodeConfig(configPath string) error {
	providers, err := s.db.ListProviders()
	if err != nil {
		return err
	}

	cfg := map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
	}

	prefs, _ := s.db.ListPreferences()

	// ── Providers ─────────────────────────────────────────────────
	provSection := make(map[string]interface{})
	for _, p := range providers {
		providerModels, _ := s.db.ListModelsByProvider(p.ID)
		whitelist := make([]string, 0, len(providerModels))
		modelEntries := make(map[string]interface{})
		for _, m := range providerModels {
			if m.Status != "error" && m.DisplayName != "" {
				whitelist = append(whitelist, m.DisplayName)
				modelEntries[m.DisplayName] = map[string]interface{}{
					"name":  m.DisplayName,
					"limit": map[string]interface{}{"context": 128000, "output": 8192},
				}
			}
		}
		entry := map[string]interface{}{}
		if len(whitelist) > 0 {
			entry["whitelist"] = whitelist
		}
		if len(modelEntries) > 0 {
			entry["models"] = modelEntries
		}
		if p.Name != "" {
			entry["name"] = p.Name
		}
		if npm, ok := prefs["config/provider_npm_"+p.ID]; ok && npm != "" && npm != "null" && npm != "NULL" {
			entry["npm"] = npm
		}
		if optsJSON, ok := prefs["config/provider_options_"+p.ID]; ok && optsJSON != "" {
			var opts map[string]interface{}
			if json.Unmarshal([]byte(optsJSON), &opts) == nil {
				entry["options"] = opts
			}
		} else if p.BaseURL != "" {
			entry["options"] = map[string]interface{}{"baseURL": p.BaseURL}
		}
		if len(entry) > 0 {
			provSection[p.ID] = entry
		}
	}
	cfg["provider"] = provSection

	// ── Agents ─────────────────────────────────────────────────────
	agents, err := s.db.ListAgents()
	if err == nil && len(agents) > 0 {
		agentSection := make(map[string]interface{})
		for _, a := range agents {
			if a.Status != "active" {
				continue
			}
			entry := map[string]interface{}{}
			if a.Description != "" {
				entry["description"] = a.Description
			}
			if a.CurrentModelID != "" {
				entry["model"] = a.CurrentModelID
			}
			if a.Mode != "" {
				entry["mode"] = a.Mode
			}
			if a.Temperature > 0 {
				entry["temperature"] = a.Temperature
			}
			if a.Color != "" {
				entry["color"] = a.Color
			}
			if a.MaxSteps > 0 {
				entry["steps"] = a.MaxSteps
			}
			if a.PromptFile != "" {
				entry["prompt"] = a.PromptFile
			}
			if a.Permission != "" {
				var permMap map[string]interface{}
				if json.Unmarshal([]byte(a.Permission), &permMap) == nil {
					entry["permission"] = permMap
				}
			}
			if len(entry) > 0 {
				agentSection[a.ID] = entry
			}
		}
		if len(agentSection) > 0 {
			cfg["agent"] = agentSection
		}
	}

	// ── Commands ──────────────────────────────────────────────────
	commands, err := s.db.ListCommands()
	if err == nil && len(commands) > 0 {
		cmdSection := make(map[string]interface{})
		for _, c := range commands {
			if c.Status != "active" {
				continue
			}
			entry := map[string]interface{}{"template": c.Template}
			if c.Description != "" {
				entry["description"] = c.Description
			}
			if c.Agent != "" {
				entry["agent"] = c.Agent
			}
			if c.Model != "" {
				entry["model"] = c.Model
			}
			if c.Subtask {
				entry["subtask"] = true
			}
			cmdSection[c.ID] = entry
		}
		cfg["command"] = cmdSection
	}

	// ── MCP ────────────────────────────────────────────────────────
	mcps, err := s.db.ListMCPs()
	if err == nil && len(mcps) > 0 {
		mcpSection := make(map[string]interface{})
		for _, m := range mcps {
			entry := map[string]interface{}{}
			if m.Type == "local" {
				entry["type"] = "local"
				if m.Command != "" {
					var cmdArr []string
					if json.Unmarshal([]byte(m.Command), &cmdArr) == nil {
						entry["command"] = cmdArr
					}
				}
			} else if m.Type == "remote" {
				entry["type"] = "remote"
				if m.URL != "" {
					entry["url"] = m.URL
				}
			}
			if m.Enabled {
				entry["enabled"] = true
			}
			if m.Timeout > 0 {
				entry["timeout"] = m.Timeout
			}
			if m.EnvVars != "" {
				var envObj map[string]interface{}
				if json.Unmarshal([]byte(m.EnvVars), &envObj) == nil {
					entry["environment"] = envObj
				}
			}
			if len(entry) > 0 {
				mcpSection[m.ID] = entry
			}
		}
		cfg["mcp"] = mcpSection
	}

	// ── Meta preferences ─────────────────────────────────────────
	for k, v := range prefs {
		if !strings.HasPrefix(k, metaPref) {
			continue
		}
		stem := k[len(metaPref):]
		// Skip provider-specific and internal keys
		if strings.HasPrefix(stem, "provider_") {
			continue
		}
		if strings.HasPrefix(stem, "skills_") {
			continue
		}
		if strings.HasPrefix(stem, "compaction_") {
			continue
		}
		if stem == "lsp" {
			continue
		}
		var val interface{}
		_ = json.Unmarshal([]byte(v), &val)
		cfg[stem] = val
	}

	// ── Write ──────────────────────────────────────────────────────
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0644)
}
