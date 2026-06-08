package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// collectKeyCandidates gathers key names from opencode.env, provider key_env in the DB,
// and OpenCode's own auth.json credentials. Also injects auth.json keys into the
// process environment so resolveKey can find them.
func collectKeyCandidates() []string {
	seen := make(map[string]bool)
	var keys []string

	if envVars, err := parseEnvFile(OpenCodeEnvPath()); err == nil {
		for k := range envVars {
			if !seen[k] {
				keys = append(keys, k)
				seen[k] = true
			}
		}
	}

	if d, err := openDB(nil); err == nil {
		providers, listErr := d.ListProviders()
		if listErr == nil {
			InjectKeysFromAuth(providers)

			for _, p := range providers {
				if p.KeyEnv != "" && !seen[p.KeyEnv] {
					keys = append(keys, p.KeyEnv)
					seen[p.KeyEnv] = true
				}
			}
		}
		d.Close()
	}

	sort.Strings(keys)
	return keys
}

// resolveKey returns the value for a key name, checking opencode.env first,
// then falling back to the process environment.
func resolveKey(key string) string {
	if envVars, err := parseEnvFile(OpenCodeEnvPath()); err == nil {
		if v := envVars[key]; v != "" {
			return v
		}
	}
	return os.Getenv(key)
}

func parseEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vars := make(map[string]string)
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
		vars[key] = val
	}
	return vars, nil
}

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
	}

	cmd.AddCommand(newKeysListCmd())
	cmd.AddCommand(newKeysSetCmd())
	cmd.AddCommand(newKeysRemoveCmd())
	cmd.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Verify API keys exist and are non-empty",
		Long: `Checks keys from opencode.env and provider key_env references.
No hardcoded key list — adapts to your actual configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			candidates := collectKeyCandidates()

			if len(candidates) == 0 {
				fmt.Println("=== API Key Check ===")
				fmt.Println()
				fmt.Println("  No keys found to check.")
				fmt.Println("  Use 'okit keys set KEY_NAME VALUE' to add keys.")
				return nil
			}

			found := 0
			missing := 0
			empty := 0

			fmt.Println("=== API Key Check ===")
			fmt.Println()

			for _, name := range candidates {
				val := resolveKey(name)

				if val == "" {
					fmt.Printf("  MISS  %s\n", name)
					missing++
				} else if val == `"` || val == `''` {
					fmt.Printf("  EMPTY %s\n", name)
					empty++
				} else {
					masked := maskKey(val)
					fmt.Printf("  OK    %s = %s\n", name, masked)
					found++
				}
			}

			fmt.Println()
			fmt.Printf("Found: %d  Missing: %d  Empty: %d\n", found, missing, empty)

			if missing > 0 || empty > 0 {
				return fmt.Errorf("%d key(s) missing or empty", missing+empty)
			}
			return nil
		},
	})

	return cmd
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func envFilePath() string {
	return OpenCodeEnvPath()
}

func newKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all API keys (masked)",
		RunE: func(cmd *cobra.Command, args []string) error {
			envPath := envFilePath()
			vars, err := parseEnvFile(envPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", envPath, err)
			}

			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			fmt.Printf("Keys in %s:\n\n", envPath)
			for _, k := range keys {
				masked := maskKey(vars[k])
				fmt.Printf("  %s=%s\n", k, masked)
			}
			fmt.Printf("\n%d key(s) total\n", len(keys))
			return nil
		},
	}
}

func newKeysSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY_NAME VALUE",
		Short: "Set or create an API key in opencode.env",
		Args:  cobra.ExactArgs(2),
		Long: `Set a key in opencode.env. Creates or updates the entry.

Security: File is written with 0600 permissions so only the owner can read.

Example:
  okit keys set MISTRAL_API_KEY "my-api-key-value"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			value := strings.TrimSpace(args[1])
			if key == "" || value == "" {
				return fmt.Errorf("key name and value must not be empty")
			}

			envPath := envFilePath()
			vars, _ := parseEnvFile(envPath)
			if vars == nil {
				vars = make(map[string]string)
			}

			vars[key] = value

			var lines []string
			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lines = append(lines, fmt.Sprintf("%s=%s", k, vars[k]))
			}

			if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
				return fmt.Errorf("writing %s: %w", envPath, err)
			}
			fmt.Printf("Set %s in %s\n", key, envPath)
			return nil
		},
	}
}

func newKeysRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove KEY_NAME",
		Short: "Remove an API key from opencode.env",
		Args:  cobra.ExactArgs(1),
		Long: `Remove a key from opencode.env.

Example:
  okit keys remove OLD_API_KEY`,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			envPath := envFilePath()
			vars, err := parseEnvFile(envPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", envPath, err)
			}

			if _, exists := vars[target]; !exists {
				return fmt.Errorf("key %q not found in %s", target, envPath)
			}
			delete(vars, target)

			var lines []string
			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lines = append(lines, fmt.Sprintf("%s=%s", k, vars[k]))
			}

			if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
				return fmt.Errorf("writing %s: %w", envPath, err)
			}
			fmt.Printf("Removed %s from %s\n", target, envPath)
			return nil
		},
	}
}
