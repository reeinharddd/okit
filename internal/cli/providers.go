package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/internal/generator"
	"github.com/reeinharddd/okit/pkg/models"
)

func newProvidersCmdImpl(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Manage providers (DB auto-sync)",
		Long: `Manage providers. Every change automatically syncs to opencode config
so both DB and config stay in sync.

Security: only the env var name is stored (e.g. MISTRAL_API_KEY),
never the actual API key value.`,
	}

	cmd.AddCommand(newProviderListCmd(dbPath))
	cmd.AddCommand(newProviderAddCmd(dbPath))
	cmd.AddCommand(newProviderUpdateCmd(dbPath))
	cmd.AddCommand(newProviderRemoveCmd(dbPath))
	cmd.PersistentFlags().String("id", "", "Provider ID")
	cmd.PersistentFlags().String("name", "", "Display name")
	cmd.PersistentFlags().String("api-base", "", "API base URL")
	cmd.PersistentFlags().String("key-env", "", "Env var name for API key")
	cmd.PersistentFlags().String("catalog-url", "", "Catalog/models endpoint URL")
	cmd.PersistentFlags().Int("priority", 99, "Provider priority")
	cmd.PersistentFlags().String("status", "active", "Provider status")
	cmd.PersistentFlags().Bool("enabled", true, "Provider enabled")
	cmd.PersistentFlags().Int("timeout-ms", 0, "Request timeout in ms")
	cmd.PersistentFlags().Int("header-timeout-ms", 0, "Header timeout in ms")
	cmd.PersistentFlags().Int("chunk-timeout-ms", 0, "Stream chunk timeout in ms")
	cmd.PersistentFlags().String("enterprise-url", "", "GitHub Enterprise URL")
	cmd.PersistentFlags().Bool("set-cache-key", false, "Enable prompt cache key")
	cmd.PersistentFlags().String("api-package", "", "NPM package name")
	cmd.PersistentFlags().String("env-list", "", "Env vars JSON array")

	return cmd
}

func newProviderListCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			providers, err := d.ListProviders()
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-25s %-15s %-8s %-7s %-7s\n", "ID", "Base URL", "Key Env", "Priority", "Status", "Enabled")
			fmt.Println(strings.Repeat("-", 90))
			for _, p := range providers {
				en := "yes"
				if !p.Enabled {
					en = "no"
				}
				fmt.Printf("%-20s %-25s %-15s %-8d %-7s %-7s\n", p.ID, p.BaseURL, p.KeyEnv, p.Priority, p.Status, en)
			}
			return nil
		},
	}
}

func newProviderAddCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a custom provider",
		Long: `Add a provider and auto-sync to opencode config.
Security: store only the env var name, NOT the key value.

Example:
  okit providers add --id my-provider --api-base https://api.example.com --key-env MY_API_KEY`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			apiBase, _ := cmd.Flags().GetString("api-base")
			keyEnv, _ := cmd.Flags().GetString("key-env")
			if id == "" || apiBase == "" || keyEnv == "" {
				return fmt.Errorf("required: --id, --api-base, --key-env")
			}

			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			p := &models.Provider{
				ID:       id,
				Name:     id,
				BaseURL:  apiBase,
				KeyEnv:   keyEnv,
				Source:   "custom",
				Status:   "active",
				Enabled:  true,
				Priority: 99,
			}

			if name, _ := cmd.Flags().GetString("name"); name != "" {
				p.Name = name
			}
			if cu, _ := cmd.Flags().GetString("catalog-url"); cu != "" {
				p.CatalogURL = cu
			}
			if pri, _ := cmd.Flags().GetInt("priority"); pri != 99 {
				p.Priority = pri
			}
			if st, _ := cmd.Flags().GetString("status"); st != "active" {
				p.Status = st
			}
			if en, _ := cmd.Flags().GetBool("enabled"); cmd.Flags().Changed("enabled") {
				p.Enabled = en
			}
			if v, _ := cmd.Flags().GetInt("timeout-ms"); v > 0 {
				p.TimeoutMs = v
			}
			if v, _ := cmd.Flags().GetInt("header-timeout-ms"); v > 0 {
				p.HeaderTimeoutMs = v
			}
			if v, _ := cmd.Flags().GetInt("chunk-timeout-ms"); v > 0 {
				p.ChunkTimeoutMs = v
			}
			if v, _ := cmd.Flags().GetString("enterprise-url"); v != "" {
				p.EnterpriseURL = v
			}
			if v, _ := cmd.Flags().GetBool("set-cache-key"); cmd.Flags().Changed("set-cache-key") {
				p.SetCacheKey = v
			}
			if v, _ := cmd.Flags().GetString("api-package"); v != "" {
				p.APIPackage = v
			}
			if v, _ := cmd.Flags().GetString("env-list"); v != "" {
				p.EnvList = v
			}

			if err := d.UpsertProvider(p); err != nil {
				return fmt.Errorf("db insert: %w", err)
			}
			fmt.Printf("Provider %s added to database.\n", id)

			return syncConfig(d)
		},
	}
}

func newProviderUpdateCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update an existing provider",
		Long: `Update provider fields. Only provided flags are changed.
Auto-syncs to opencode config.

Example:
  okit providers update --id my-provider --api-base https://new-api.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("required: --id")
			}

			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			existing, err := d.GetProvider(id)
			if err != nil {
				return fmt.Errorf("provider %q not found: %w", id, err)
			}

			changed := false
			if name, _ := cmd.Flags().GetString("name"); cmd.Flags().Changed("name") {
				existing.Name = name
				changed = true
			}
			if apiBase, _ := cmd.Flags().GetString("api-base"); cmd.Flags().Changed("api-base") {
				existing.BaseURL = apiBase
				changed = true
			}
			if keyEnv, _ := cmd.Flags().GetString("key-env"); cmd.Flags().Changed("key-env") {
				existing.KeyEnv = keyEnv
				changed = true
			}
			if cu, _ := cmd.Flags().GetString("catalog-url"); cmd.Flags().Changed("catalog-url") {
				existing.CatalogURL = cu
				changed = true
			}
			if pri, _ := cmd.Flags().GetInt("priority"); cmd.Flags().Changed("priority") {
				existing.Priority = pri
				changed = true
			}
			if st, _ := cmd.Flags().GetString("status"); cmd.Flags().Changed("status") {
				existing.Status = st
				changed = true
			}
			if en, _ := cmd.Flags().GetBool("enabled"); cmd.Flags().Changed("enabled") {
				existing.Enabled = en; changed = true
			}
			if v, _ := cmd.Flags().GetInt("timeout-ms"); cmd.Flags().Changed("timeout-ms") {
				existing.TimeoutMs = v; changed = true
			}
			if v, _ := cmd.Flags().GetInt("header-timeout-ms"); cmd.Flags().Changed("header-timeout-ms") {
				existing.HeaderTimeoutMs = v; changed = true
			}
			if v, _ := cmd.Flags().GetInt("chunk-timeout-ms"); cmd.Flags().Changed("chunk-timeout-ms") {
				existing.ChunkTimeoutMs = v; changed = true
			}
			if v, _ := cmd.Flags().GetString("enterprise-url"); cmd.Flags().Changed("enterprise-url") {
				existing.EnterpriseURL = v; changed = true
			}
			if v, _ := cmd.Flags().GetBool("set-cache-key"); cmd.Flags().Changed("set-cache-key") {
				existing.SetCacheKey = v; changed = true
			}
			if v, _ := cmd.Flags().GetString("api-package"); cmd.Flags().Changed("api-package") {
				existing.APIPackage = v; changed = true
			}
			if v, _ := cmd.Flags().GetString("env-list"); cmd.Flags().Changed("env-list") {
				existing.EnvList = v; changed = true
			}

			if !changed {
				return fmt.Errorf("no fields to update (specify at least one flag)")
			}

			if err := d.UpsertProvider(existing); err != nil {
				return fmt.Errorf("db update: %w", err)
			}
			fmt.Printf("Provider %s updated in database.\n", id)

			return syncConfig(d)
		},
	}
}

func newProviderRemoveCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove a provider",
		Long: `Remove a provider and its models from DB.
Auto-syncs to opencode config.

Example:
  okit providers remove --id my-provider`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("required: --id")
			}

			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

		prov, err := d.GetProvider(id)
		if err != nil {
			return fmt.Errorf("provider %q not found", id)
		}
		if err := d.DeleteProvider(id); err != nil {
			return fmt.Errorf("db delete: %w", err)
		}
		if prov.Source == "seed" {
			_ = d.SetPreference("seed_removed:"+id, "1")
		}
		fmt.Printf("Provider %s removed from database.\n", id)

		return syncConfig(d)
		},
	}
}

func syncConfig(d *db.DB) error {
	configDir := filepath.Dir(d.DBPath())
	gen := generator.NewService(d, configDir)
	if err := gen.GenerateConfig(); err != nil {
		return fmt.Errorf("sync config: %w", err)
	}
	return nil
}
