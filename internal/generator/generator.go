package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

const metaPrefix = "config/"

type Service struct {
	db        db.DBInterface
	outputDir string
}

func NewService(db db.DBInterface, outputDir string) *Service {
	if outputDir == "" {
		outputDir = filepath.Dir(db.DBPath())
	}
	return &Service{db: db, outputDir: outputDir}
}

const (
	defaultContextWindow = 128000
	defaultMaxOutput     = 8192
)

func (s *Service) GenerateConfig() error {
	if err := s.SyncExistingToDB(); err != nil {
		fmt.Printf("  Warning: sync existing config: %v\n", err)
	}

	providers, err := s.db.ListProviders()
	if err != nil {
		return fmt.Errorf("providers: %w", err)
	}

	profiles, err := s.loadProfiles()
	if err != nil {
		return fmt.Errorf("profiles: %w", err)
	}

	cfg := map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
	}

	totalActive, totalError, providerSection := s.buildProviderSection(providers, profiles)
	if len(providerSection) > 0 {
		cfg["provider"] = providerSection
	}

	if section, err := s.buildAgentSection(); err == nil {
		cfg["agent"] = section
	}

	cfg["permission"] = map[string]interface{}{}
	cfg["experimental"] = map[string]interface{}{}

	if err := s.writeStateFile(totalActive, totalError, len(providerSection)); err != nil {
		return fmt.Errorf("state file: %w", err)
	}

	if section, err := s.buildCommandSection(); err == nil {
		cfg["command"] = section
	}

	if section, err := s.buildMCPSection(); err == nil {
		cfg["mcp"] = section
	}

	meta := s.buildMetaFromDB()
	for k, v := range meta {
		cfg[k] = v
	}

	merged := s.mergeWithExisting(cfg, s.readExistingConfig())

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	configName := "opencode.jsonc"
	if _, err := os.Stat(filepath.Join(s.outputDir, "opencode.json")); err == nil {
		configName = "opencode.json"
	}
	configPath := filepath.Join(s.outputDir, configName)
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fmt.Printf("  Generated: %s\n", configPath)
	return nil
}

func (s *Service) SyncExistingToDB() error {
	cfg := s.readExistingConfig()
	if cfg == nil {
		return nil
	}

	existingProviders, _ := s.db.ListProviders()

	// Index existing by base URL (skip empty base URLs)
	byBaseURL := make(map[string]string) // baseURL → provider ID
	for _, p := range existingProviders {
		if p.BaseURL != "" {
			byBaseURL[p.BaseURL] = p.ID
		}
	}

	syncAgents := func() {
		agentSection, ok := cfg["agent"].(map[string]interface{})
		if !ok {
			return
		}
		for agentID, agentVal := range agentSection {
			agentMap, ok := agentVal.(map[string]interface{})
			if !ok {
				continue
			}
			model, _ := agentMap["model"].(string)
			desc, _ := agentMap["description"].(string)
			mode, _ := agentMap["mode"].(string)
			modeVal := mode
			if modeVal == "" {
				modeVal = "subagent"
			}
			t, _ := agentMap["temperature"].(float64)
			color, _ := agentMap["color"].(string)
			steps, _ := agentMap["steps"].(float64)
			promptFile, _ := agentMap["prompt"].(string)

			var permBytes []byte
			if perm, ok := agentMap["permission"]; ok {
				permBytes, _ = json.Marshal(perm)
			}

			_ = s.db.UpsertAgent(&models.Agent{
				ID:             agentID,
				Description:    desc,
				CurrentModelID: model,
				Mode:           modeVal,
				Temperature:    t,
				Color:          color,
				MaxSteps:       int(steps),
				PromptFile:     promptFile,
				Permission:     string(permBytes),
				Source:         "opencode",
				Status:         "active",
			})
		}
	}

	syncProviders := func() {
		provSection, ok := cfg["provider"].(map[string]interface{})
		if !ok {
			return
		}
		for provID, provVal := range provSection {
			provCfg, ok := provVal.(map[string]interface{})
			if !ok {
				continue
			}

			// Read config entry
			name, _ := provCfg["name"].(string)
			p := models.Provider{
				ID:       provID,
				Name:     name,
				Source:   "opencode",
				Status:   "active",
				Enabled:  true,
			}
			if opts, ok := provCfg["options"].(map[string]interface{}); ok {
				if bu, _ := opts["baseURL"].(string); bu != "" {
					p.BaseURL = bu
				}
				if to, _ := opts["timeout"].(float64); to > 0 {
					p.TimeoutMs = int(to)
				}
				if hto, _ := opts["headerTimeout"].(float64); hto > 0 {
					p.HeaderTimeoutMs = int(hto)
				}
				if cto, _ := opts["chunkTimeout"].(float64); cto > 0 {
					p.ChunkTimeoutMs = int(cto)
				}
				if eu, _ := opts["enterpriseUrl"].(string); eu != "" {
					p.EnterpriseURL = eu
				}
				if sck, _ := opts["setCacheKey"].(bool); sck {
					p.SetCacheKey = true
				}
			}

			// If this config entry's base URL matches an existing provider with a
			// different ID (e.g. config has "opencode", seed has "opencode-zen"),
			// merge into the existing one instead of creating a duplicate.
			if p.BaseURL != "" {
				if existingID, ok := byBaseURL[p.BaseURL]; ok && existingID != provID {
					if existing, err := s.db.GetProvider(existingID); err == nil {
						p.ID = existingID
						if p.Name == "" {
							p.Name = existing.Name
						}
						if existing.KeyEnv != "" {
							p.KeyEnv = existing.KeyEnv
							p.CatalogURL = existing.CatalogURL
						}
						// Merge any config-provided fields into the existing record
						if name != "" {
							p.Name = name
						}
						_ = s.db.UpsertProvider(&p)
						continue
					}
				}
			}

			// Preserve existing KeyEnv — don't wipe what discover set
			if existing, err := s.db.GetProvider(provID); err == nil && existing.KeyEnv != "" {
				p.KeyEnv = existing.KeyEnv
				p.CatalogURL = existing.CatalogURL
			}
			_ = s.db.UpsertProvider(&p)
		}
	}

	syncMCP := func() {
		mcpRaw, ok := cfg["mcp"].(map[string]interface{})
		if !ok {
			return
		}
		for id, val := range mcpRaw {
			entry, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			m := models.MCPServer{ID: id, Source: "sync"}
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
			if e, _ := entry["env"].(map[string]interface{}); len(e) > 0 {
				b, _ := json.Marshal(e)
				m.EnvVars = string(b)
			}
			if enabled, _ := entry["enabled"].(bool); enabled {
				m.Enabled = true
			}
			if timeout, _ := entry["timeout"].(float64); timeout > 0 {
				m.Timeout = int(timeout)
			}
			_ = s.db.UpsertMCP(&m)
		}
	}

	syncCommands := func() {
		cmdSection, ok := cfg["command"].(map[string]interface{})
		if !ok {
			return
		}
		for cmdID, cmdVal := range cmdSection {
			cmdMap, ok := cmdVal.(map[string]interface{})
			if !ok {
				continue
			}
			template, _ := cmdMap["template"].(string)
			desc, _ := cmdMap["description"].(string)
			agent, _ := cmdMap["agent"].(string)
			model, _ := cmdMap["model"].(string)

			isSubtask := false
			if v, ok := cmdMap["subtask"]; ok {
				isSubtask = v.(bool)
			}

			_ = s.db.UpsertCommand(&models.Command{
				ID:          cmdID,
				Template:    template,
				Description: desc,
				Agent:       agent,
				Model:       model,
				Subtask:     isSubtask,
				Source:      "opencode",
				Status:      "active",
			})
		}
	}

	syncAgents()
	syncProviders()
	syncMCP()
	syncCommands()

	// Sync meta preferences
	metaKeys := map[string]bool{
		"autoupdate": true, "disabled_providers": true, "model": true,
		"small_model": true, "share": true, "compaction": true, "lsp": true,
		"skills": true, "enabled_providers": true,
	}
	for key := range metaKeys {
		val, ok := cfg[key]
		if !ok {
			continue
		}
		b, err := json.Marshal(val)
		if err != nil {
			continue
		}
		_ = s.db.SetPreference(metaPrefix+key, string(b))
	}

	return nil
}

func (s *Service) buildProviderSection(providers []models.Provider, profiles map[string]models.ModelProfile) (int, int, map[string]interface{}) {
	section := make(map[string]interface{})
	totalActive := 0
	totalError := 0
	prefs, _ := s.db.ListPreferences()

	// Read disabled_providers / enabled_providers from preferences
	disabledRaw, _ := prefs["config/disabled_providers"]
	disabled := parseStringList(disabledRaw)
	enabledRaw, _ := prefs["config/enabled_providers"]
	enabled := parseStringList(enabledRaw)
	hasEnabledFilter := len(enabled) > 0

	for _, p := range providers {
		// Explicit disabled/enabled filtering
		if _, disabled := disabled[p.ID]; disabled {
			continue
		}
		if hasEnabledFilter && !enabled[p.ID] {
			continue
		}

		models, err := s.db.ListModelsByProvider(p.ID)
		if err != nil {
			continue
		}

		whitelist := make([]string, 0, len(models))
		modelEntries := make(map[string]interface{})
		for _, m := range models {
			if m.Status == "error" {
				totalError++
				continue
			}
			totalActive++
			whitelist = append(whitelist, m.DisplayName)
			entry := buildModelEntry(m, profiles[m.ID])
			modelEntries[m.DisplayName] = entry
		}

		// Skip providers with no models (regardless of key status)
		if len(whitelist) == 0 {
			continue
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

		// Provider options
		opts := map[string]interface{}{}
		if p.BaseURL != "" {
			opts["baseURL"] = p.BaseURL
		}
		if p.TimeoutMs > 0 {
			opts["timeout"] = p.TimeoutMs
		}
		if p.HeaderTimeoutMs > 0 {
			opts["headerTimeout"] = p.HeaderTimeoutMs
		}
		if p.ChunkTimeoutMs > 0 {
			opts["chunkTimeout"] = p.ChunkTimeoutMs
		}
		if p.EnterpriseURL != "" {
			opts["enterpriseUrl"] = p.EnterpriseURL
		}
		if p.SetCacheKey {
			opts["setCacheKey"] = true
		}
		if len(opts) > 0 {
			entry["options"] = opts
		}

		if len(entry) > 0 {
			section[p.ID] = entry
		}
	}
	return totalActive, totalError, section
}

func parseStringList(raw string) map[string]bool {
	out := make(map[string]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return out
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return out
	}
	for _, s := range list {
		out[s] = true
	}
	return out
}

func buildModelEntry(m models.Model, profile models.ModelProfile) map[string]interface{} {
	context := m.ContextWindow
	if context <= 0 {
		context = defaultContextWindow
	}
	maxOutput := m.MaxOutput
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutput
	}
	if profile.MaxOutput > 0 {
		maxOutput = profile.MaxOutput
	}

	entry := map[string]interface{}{}
	if m.DisplayName != "" {
		entry["name"] = m.DisplayName
	}
	if m.Description != "" {
		entry["description"] = m.Description
	}
	if m.Family != "" {
		entry["family"] = m.Family
	}
	if m.ReleaseDate != "" {
		entry["release_date"] = m.ReleaseDate
	}
	if m.Aliases != "" {
		var aliases []string
		if err := json.Unmarshal([]byte(m.Aliases), &aliases); err == nil {
			entry["aliases"] = aliases
		}
	}
	if m.Experimental {
		entry["experimental"] = true
	}
	if m.Interleaved != "" {
		var interleaved interface{}
		if err := json.Unmarshal([]byte(m.Interleaved), &interleaved); err == nil {
			entry["interleaved"] = interleaved
		}
	}

	limit := map[string]interface{}{
		"context": context,
	}
	if maxOutput > 0 {
		limit["output"] = maxOutput
	}
	entry["limit"] = limit

	if m.FunctionCalling {
		entry["tool_call"] = true
	}
	if m.Reasoning {
		entry["reasoning"] = true
	}
	if m.DefaultTemp > 0 {
		entry["temperature"] = true
	}

	// Modalities
	if m.Vision || m.Audio || m.OCR || m.ModalitiesInput != "" || m.ModalitiesOutput != "" {
		modalities := map[string]interface{}{}
		if m.ModalitiesInput != "" {
			var in []string
			if err := json.Unmarshal([]byte(m.ModalitiesInput), &in); err == nil {
				modalities["input"] = in
			}
		} else if m.Vision || m.OCR {
			modalities["input"] = []string{"text", "image"}
		}
		if m.ModalitiesOutput != "" {
			var out []string
			if err := json.Unmarshal([]byte(m.ModalitiesOutput), &out); err == nil {
				modalities["output"] = out
			}
		}
		if m.Vision {
			entry["attachment"] = true
		}
		if len(modalities) > 0 {
			entry["modalities"] = modalities
		}
	}

	// Cost
	hasCost := m.PricingPrompt > 0 || m.PricingCompletion > 0 || m.PricingCacheRead > 0 || m.PricingCacheWrite > 0
	if hasCost {
		cost := map[string]interface{}{
			"input":  m.PricingPrompt,
			"output": m.PricingCompletion,
		}
		if m.PricingCacheRead > 0 {
			cost["cache_read"] = m.PricingCacheRead
		}
		if m.PricingCacheWrite > 0 {
			cost["cache_write"] = m.PricingCacheWrite
		}
		entry["cost"] = cost
	}

	// Status mapping: OpenCode schema accepts alpha, beta, deprecated, active
	if m.Status != "" && m.Status != "active" && m.Status != "untested" {
		status := m.Status
		if status == "paid" {
			status = "alpha"
		}
		entry["status"] = status
	}
	if m.Deprecation != "" {
		var dep interface{}
		if err := json.Unmarshal([]byte(m.Deprecation), &dep); err == nil {
			entry["deprecation"] = dep
		}
	}

	return entry
}

func (s *Service) loadProfiles() (map[string]models.ModelProfile, error) {
	profiles, err := s.db.ListModelProfiles()
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.ModelProfile, len(profiles))
	for _, p := range profiles {
		out[p.ModelID] = p
	}
	return out, nil
}

func (s *Service) buildAgentSection() (map[string]interface{}, error) {
	agents, err := s.db.ListAgents()
	if err != nil {
		return nil, err
	}
	section := make(map[string]interface{})
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
			if err := json.Unmarshal([]byte(a.Permission), &permMap); err == nil {
				entry["permission"] = permMap
			}
		}
		if len(entry) > 0 {
			section[a.ID] = entry
		}
	}
	return section, nil
}

func (s *Service) buildCommandSection() (map[string]interface{}, error) {
	commands, err := s.db.ListCommands()
	if err != nil {
		return nil, err
	}
	section := make(map[string]interface{})
	for _, c := range commands {
		if c.Status != "active" {
			continue
		}
		entry := map[string]interface{}{
			"template": c.Template,
		}
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
		section[c.ID] = entry
	}
	return section, nil
}

func (s *Service) buildMCPSection() (map[string]interface{}, error) {
	mcps, err := s.db.ListMCPs()
	if err != nil {
		return nil, err
	}
	section := make(map[string]interface{})
	for _, m := range mcps {
		entry := map[string]interface{}{}
		if m.Type == "local" {
			entry["type"] = "local"
			if m.Command != "" {
				var cmdArr []string
				if err := json.Unmarshal([]byte(m.Command), &cmdArr); err == nil {
					entry["command"] = cmdArr
				}
			}
		} else if m.Type == "remote" {
			entry["type"] = "remote"
			if m.URL != "" {
				entry["url"] = m.URL
			}
		}
		entry["enabled"] = m.Enabled
		if m.Timeout > 0 {
			entry["timeout"] = m.Timeout
		}
		if m.EnvVars != "" {
			var envObj map[string]interface{}
			if err := json.Unmarshal([]byte(m.EnvVars), &envObj); err == nil {
				entry["environment"] = envObj
			}
		}
		if len(entry) > 0 {
			section[m.ID] = entry
		}
	}
	return section, nil
}

func (s *Service) writeStateFile(active, errored, providers int) error {
	state := map[string]interface{}{
		"active_models": active,
		"error_models":  errored,
		"providers":     providers,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"source":        "opencode-kit",
	}
	if routes, err := s.db.ListRoutingRules(); err == nil && len(routes) > 0 {
		routeEntries := make(map[string]interface{})
		for _, r := range routes {
			entry := map[string]interface{}{}
			if r.CurrentModelID != "" {
				entry["model"] = r.CurrentModelID
			}
			if r.FallbackIDs != "" {
				entry["fallback"] = r.FallbackIDs
			}
			if r.Description != "" {
				entry["description"] = r.Description
			}
			if r.MinContext > 0 {
				entry["min_context"] = r.MinContext
			}
			if r.NeedsFC {
				entry["needs_fc"] = true
			}
			if r.NeedsVision {
				entry["needs_vision"] = true
			}
			if r.MaxCostPerCall > 0 {
				entry["max_cost"] = r.MaxCostPerCall
			}
			routeEntries[r.TaskKey] = entry
		}
		state["routes"] = routeEntries
	}
	if skills, err := s.db.ListSkills(); err == nil && len(skills) > 0 {
		skillEntries := make([]string, 0, len(skills))
		for _, sk := range skills {
			if sk.Status == "active" {
				skillEntries = append(skillEntries, sk.ID)
			}
		}
		state["skills"] = skillEntries
	}

	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.outputDir, "okit-state.json")
	if err := os.WriteFile(path, out, 0644); err != nil {
		return err
	}
	fmt.Printf("  Generated: %s\n", path)
	return nil
}

func (s *Service) GenerateAgents() error {
	agents, err := s.db.ListAgents()
	if err != nil {
		return err
	}

	agentsDir := filepath.Join(s.outputDir, "agents")
	os.MkdirAll(agentsDir, 0755)

	for _, a := range agents {
		if a.Status != "active" {
			continue
		}
		path := filepath.Join(agentsDir, a.ID+".md")
		fm := fmt.Sprintf("---\ndescription: %s\nmode: %s\n", a.Description, a.Mode)
		if a.CurrentModelID != "" {
			fm += fmt.Sprintf("model: %s\n", a.CurrentModelID)
		}
		if a.Temperature > 0 {
			fm += fmt.Sprintf("temperature: %.1f\n", a.Temperature)
		}
		if a.Color != "" {
			fm += fmt.Sprintf("color: %s\n", a.Color)
		}
		if a.Permission != "" {
			fm += fmt.Sprintf("permission:\n")
			var permMap map[string]interface{}
			if err := json.Unmarshal([]byte(a.Permission), &permMap); err == nil {
				for k, v := range permMap {
					fm += fmt.Sprintf("  %s: %v\n", k, v)
				}
			}
		}
		fm += "---\n\n"

		prompt := "# " + a.Description + "\n"
		if a.PromptFile != "" {
			prompt += "\n" + a.PromptFile + "\n"
		}
		content := fm + prompt

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write agent %s: %w", a.ID, err)
		}
		fmt.Printf("  Agent: %s\n", path)
	}
	return nil
}

func (s *Service) GenerateCommands() error {
	commands, err := s.db.ListCommands()
	if err != nil {
		return err
	}

	cmdsDir := filepath.Join(s.outputDir, "commands")
	os.MkdirAll(cmdsDir, 0755)

	for _, c := range commands {
		if c.Status != "active" {
			continue
		}
		path := filepath.Join(cmdsDir, c.ID+".md")
		fm := "---\n"
		if c.Description != "" {
			fm += fmt.Sprintf("description: %s\n", c.Description)
		}
		if c.Agent != "" {
			fm += fmt.Sprintf("agent: %s\n", c.Agent)
		}
		if c.Model != "" {
			fm += fmt.Sprintf("model: %s\n", c.Model)
		}
		fm += "---\n\n"

		tpl := c.Template
		if tpl == "" {
			tpl = "# " + c.Description
		}
		content := fm + tpl + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write command %s: %w", c.ID, err)
		}
		fmt.Printf("  Command: %s\n", path)
	}
	return nil
}

func (s *Service) readExistingConfig() map[string]interface{} {
	configName := "opencode.jsonc"
	if _, err := os.Stat(filepath.Join(s.outputDir, "opencode.json")); err == nil {
		configName = "opencode.json"
	}
	path := filepath.Join(s.outputDir, configName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg
}

func (s *Service) buildMetaFromDB() map[string]interface{} {
	prefs, err := s.db.ListPreferences()
	if err != nil {
		return nil
	}
	result := make(map[string]interface{})
	compactionKeys := make(map[string]interface{})

	for k, v := range prefs {
		if !strings.HasPrefix(k, metaPrefix) {
			continue
		}
		stem := k[len(metaPrefix):]

		if strings.HasPrefix(stem, "compaction_") {
			var val interface{}
			_ = json.Unmarshal([]byte(v), &val)
			compactionKeys[stem[len("compaction_"):]] = val
			continue
		}
		if strings.HasPrefix(stem, "skills_") {
			sk := stem[len("skills_"):]
			if _, exists := result["skills"]; !exists {
				result["skills"] = make(map[string]interface{})
			}
			obj := result["skills"].(map[string]interface{})
			var val interface{}
			_ = json.Unmarshal([]byte(v), &val)
			obj[sk] = val
			continue
		}
		if strings.HasPrefix(stem, "provider_") {
			continue
		}
		var val interface{}
		_ = json.Unmarshal([]byte(v), &val)
		result[stem] = val
	}
	if len(compactionKeys) > 0 {
		result["compaction"] = compactionKeys
	}
	return result
}

var generatorManagedKeys = map[string]bool{
	"$schema":            true,
	"provider":           true,
	"agent":              true,
	"command":            true,
	"mcp":                true,
	"permission":         true,
	"experimental":       true,
	"lsp":                true,
	"plugin":             true,
	"skills":             true,
	"autoupdate":         true,
	"disabled_providers": true,
	"model":              true,
	"small_model":        true,
	"share":              true,
	"compaction":         true,
}

func (s *Service) mergeWithExisting(generated, existing map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range existing {
		if !generatorManagedKeys[k] {
			result[k] = v
		}
	}
	for k, v := range generated {
		if k == "mcp" {
			genMcp, _ := v.(map[string]interface{})
			existingMcp, _ := existing["mcp"].(map[string]interface{})
			result["mcp"] = mergeMCP(genMcp, existingMcp)
			continue
		}
		result[k] = v
	}
	return result
}

func mergeMCP(generated, existing map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(generated)+len(existing))
	for k, v := range generated {
		merged[k] = v
	}
	for k, v := range existing {
		if _, covered := merged[k]; !covered {
			merged[k] = v
		}
	}
	return merged
}
