package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/reeinharddd/okit/internal/config"
	"github.com/reeinharddd/okit/pkg/models"
)

func OpenCodeConfigDir() string {
	return config.ConfigDir()
}

func opencodeConfigName() string {
	dir := OpenCodeConfigDir()
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); err == nil {
		return "opencode.json"
	}
	return "opencode.jsonc"
}

func OpenCodeConfigPath() string {
	return filepath.Join(OpenCodeConfigDir(), opencodeConfigName())
}

func OpenCodeEnvPath() string {
	return filepath.Join(OpenCodeConfigDir(), "opencode.env")
}

func OpenCodeDBPath() string {
	return filepath.Join(OpenCodeConfigDir(), "opencode-kit.db")
}

func LoadEnvFile() error {
	path := OpenCodeEnvPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if key != "" && val != "" {
			os.Setenv(key, val)
		}
	}
	return nil
}

// InjectKeysFromAuth reads OpenCode's auth.json (~/.local/share/opencode/auth.json)
// and injects any keys that match DB providers into the process environment.
func InjectKeysFromAuth(providers []models.Provider) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var auth map[string]struct {
		Type string `json:"type"`
		Key  string `json:"key"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return
	}

	// Build a name-to-id index for fuzzy matching
	nameToID := make(map[string]string)
	for _, p := range providers {
		if p.Name != "" {
			nameToID[strings.ToLower(p.Name)] = p.ID
		}
	}

	for _, p := range providers {
		if os.Getenv(p.KeyEnv) != "" {
			continue
		}
		// Exact match by provider ID
		if entry, ok := auth[p.ID]; ok && entry.Key != "" {
			os.Setenv(p.KeyEnv, entry.Key)
			continue
		}
		// Fuzzy match: auth IDs like "opencode" → DB name "OpenCode Zen" → clean "opencodezen"
		cleanName := strings.ToLower(p.Name)
		cleanName = strings.ReplaceAll(cleanName, " ", "")
		cleanName = strings.ReplaceAll(cleanName, "-", "")
		for authID, entry := range auth {
			if entry.Key == "" {
				continue
			}
			lower := strings.ToLower(authID)
			if strings.HasPrefix(cleanName, lower) || strings.HasPrefix(lower, cleanName) {
				os.Setenv(p.KeyEnv, entry.Key)
				break
			}
		}
	}
}
