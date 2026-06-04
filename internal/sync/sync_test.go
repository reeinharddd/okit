package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

func TestImportFromOpenCodeConfig_WithAllSections(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	configPath := filepath.Join(tempDir, "opencode.jsonc")
	config := map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"provider": map[string]interface{}{
			"groq": map[string]interface{}{
				"whitelist": []string{"llama-3.3-70b-versatile"},
			},
		},
		"agent": map[string]interface{}{
			"build": map[string]interface{}{
				"model": "groq/llama-3.3-70b-versatile",
			},
		},
		"command": map[string]interface{}{
			"test": map[string]interface{}{
				"template": "test",
			},
		},
		"mcp": map[string]interface{}{
			"engram": map[string]interface{}{
				"type": "local",
				"command": []string{"engram", "mcp", "--tools=agent"},
				"enabled": true,
			},
		},
		"lsp": true,
		"autoupdate": false,
		"disabled_providers": []string{"opencode"},
		"model": "groq/llama-3.3-70b-versatile",
		"small_model": "groq/llama-3.1-8b-instant",
		"share": "manual",
		"compaction": map[string]interface{}{
			"auto": true,
		},
	}

	out, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc := New(d)
	diff, err := svc.ImportFromOpenCodeConfig(configPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	// groq is already seeded, import should not report it as added
	if len(diff.AddedProviders) != 0 {
		t.Errorf("providers: want [], got %v", diff.AddedProviders)
	}
	if len(diff.AddedAgents) != 1 || diff.AddedAgents[0] != "build" {
		t.Errorf("agents: want [build], got %v", diff.AddedAgents)
	}
	if len(diff.AddedCommands) != 1 || diff.AddedCommands[0] != "test" {
		t.Errorf("commands: want [test], got %v", diff.AddedCommands)
	}
	if len(diff.AddedMCPs) != 1 || diff.AddedMCPs[0] != "engram" {
		t.Errorf("mcps: want [engram], got %v", diff.AddedMCPs)
	}

	mcp, _ := d.ListMCPs()
	if len(mcp) != 1 || mcp[0].ID != "engram" {
		t.Errorf("mcp not imported: %v", mcp)
	}

	lsp, _ := d.ListPreferences()
	if lsp[metaPref+"lsp"] != "true" {
		t.Errorf("lsp not imported: %v", lsp[metaPref+"lsp"])
	}

	meta, _ := d.ListPreferences()
	for _, key := range []string{"autoupdate", "disabled_providers", "model", "small_model", "share"} {
		if _, ok := meta[metaPref+key]; !ok {
			t.Errorf("meta key %q not imported", key)
		}
	}

	// Verify model usage
	modelList, _ := d.ListModelsByProvider("groq")
	if len(modelList) != 1 || modelList[0].DisplayName != "llama-3.3-70b-versatile" {
		t.Errorf("model not imported: %v", modelList)
	}

	// Verify agent usage
	agents, _ := d.ListAgents()
	if len(agents) != 1 || agents[0].ID != "build" {
		t.Errorf("agent not imported: %v", agents)
	}

	// Verify command usage
	commands, _ := d.ListCommands()
	if len(commands) != 1 || commands[0].ID != "test" {
		t.Errorf("command not imported: %v", commands)
	}

	// Verify MCP usage
	mcpServers, _ := d.ListMCPs()
	if len(mcpServers) != 1 || mcpServers[0].ID != "engram" {
		t.Errorf("MCP server not imported: %v", mcpServers)
	}

	// Verify LSP usage
	lspServers, _ := d.ListLSPServers()
	if len(lspServers) != 0 {
		t.Errorf("LSP servers should not be imported when lsp is boolean, got %v", lspServers)
	}

	// Verify meta preferences
	prefs, _ := d.ListPreferences()
	for _, key := range []string{"autoupdate", "disabled_providers", "model", "small_model", "share"} {
		if _, ok := prefs[metaPref+key]; !ok {
			t.Errorf("meta preference %q not imported", key)
		}
	}

	// Verify provider usage — groq exists among seeded providers
	providers, _ := d.ListProviders()
	found := false
	for _, p := range providers {
		if p.ID == "groq" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("provider groq not found among: %v", providers)
	}

	// Verify model profile usage
	profiles, _ := d.ListModelProfiles()
	if len(profiles) != 1 || profiles[0].ModelID != "groq/llama-3.3-70b-versatile" {
		t.Errorf("model profile not imported: %v", profiles)
	}

	// Verify model instance usage
	model := models.Model{
		ID:          "groq/llama-3.3-70b-versatile",
		ProviderID:  "groq",
		DisplayName: "llama-3.3-70b-versatile",
		Status:      "active",
	}
	if err := d.UpsertModel(&model); err != nil {
		t.Errorf("failed to upsert model: %v", err)
	}

	// Verify agent instance usage
	agent := models.Agent{
		ID:             "build",
		Description:    "Build agent",
		CurrentModelID: "groq/llama-3.3-70b-versatile",
		Status:         "active",
	}
	if err := d.UpsertAgent(&agent); err != nil {
		t.Errorf("failed to upsert agent: %v", err)
	}

	// Verify command instance usage
	command := models.Command{
		ID:          "test",
		Template:    "test",
		Description: "Test command",
		Status:      "active",
	}
	if err := d.UpsertCommand(&command); err != nil {
		t.Errorf("failed to upsert command: %v", err)
	}

	// Verify MCP instance usage
	mcpServer := models.MCPServer{
		ID:      "engram",
		Type:    "local",
		Command: "[\"engram\", \"mcp\", \"--tools=agent\"]",
		Enabled: true,
	}
	if err := d.UpsertMCP(&mcpServer); err != nil {
		t.Errorf("failed to upsert MCP server: %v", err)
	}
}