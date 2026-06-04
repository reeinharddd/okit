package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reeinharddd/okit/internal/audit"
	"github.com/reeinharddd/okit/internal/compress"
	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/internal/discover"
	"github.com/reeinharddd/okit/internal/generator"
	"github.com/reeinharddd/okit/internal/heal"
	"github.com/reeinharddd/okit/internal/profile"
	"github.com/reeinharddd/okit/internal/routing"
	"github.com/reeinharddd/okit/internal/sync"
	"github.com/reeinharddd/okit/pkg/models"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var dbPath string

	root := &cobra.Command{
		Use:   "okit",
		Short: "opencode-kit - OpenCode infrastructure manager",
		Long: `opencode-kit manages OpenCode configuration: discovers models,
audits capabilities, generates optimal config, and auto-heals issues.

  okit discover     Fetch models from provider catalogs
  okit audit        Test model capabilities
  okit generate     Generate OpenCode config files
  okit daily        Full daily pipeline
  okit status       Show system status
  okit query        Run SQL query against DB`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}
			_ = LoadEnvFile()
			return nil
		},
	}

	root.PersistentFlags().StringVar(&dbPath, "db", "", "Path to SQLite database (default: auto-detect)")

	root.AddCommand(newDiscoverCmd(&dbPath))
	root.AddCommand(newAuditCmd(&dbPath))
	root.AddCommand(newGenerateCmd(&dbPath))
	root.AddCommand(newDailyCmd(&dbPath))
	root.AddCommand(newStatusCmd(&dbPath))
	root.AddCommand(newQueryCmd(&dbPath))
	root.AddCommand(newProvidersCmd(&dbPath))
	root.AddCommand(newModelsCmd(&dbPath))
	root.AddCommand(newSyncCmd(&dbPath))
	root.AddCommand(newSourcesCmd(&dbPath))
	root.AddCommand(newProfileCmd(&dbPath))
	root.AddCommand(newRouteCmd(&dbPath))
	root.AddCommand(newHealCmd(&dbPath))
	root.AddCommand(newKeysCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newMcpCmd())
	root.AddCommand(newCompressCmd())
	root.AddCommand(newValidateCmd(&dbPath))
	root.AddCommand(newModelsViewCmd(&dbPath))
	root.AddCommand(newBudgetCmd(&dbPath))
	root.AddCommand(newLSPServersCmd(&dbPath))
	root.AddCommand(newSnapshotsCmd(&dbPath))
	root.AddCommand(newPreferencesCmd(&dbPath))
	root.AddCommand(newSkillsCmd(&dbPath))
	root.AddCommand(newSourceItemsCmd(&dbPath))
	root.AddCommand(newExecLogCmd(&dbPath))
	root.AddCommand(newModelProfilesCmd(&dbPath))
	root.AddCommand(newAgentsCmd(&dbPath))

	return root
}

func openDB(path *string) (*db.DB, error) {
	p := ""
	if path != nil {
		p = *path
	}
	if p == "" {
		p = db.DefaultPath()
	}
	return db.Open(p)
}

func runHeal(d *db.DB) (*heal.HealReport, error) {
	return heal.New(d).Run(context.Background())
}

func newDiscoverCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover models from provider catalogs",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			dis := discover.NewService(discover.NewServiceParams{DB: d})
			if err := dis.Discover(cmd.Context()); err != nil {
				return fmt.Errorf("discover: %w", err)
			}
			fmt.Println("Discovery complete.")
			return nil
		},
	}
}

func newAuditCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Test model capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			full, _ := cmd.Flags().GetBool("full")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			svc := audit.New(d, 5)
			return svc.Run(cmd.Context(), full)
		},
	}
	cmd.Flags().Bool("full", false, "Test all models (default: only new + errored)")
	cmd.AddCommand(newAuditLiveCmd(dbPath))
	return cmd
}

func newGenerateCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate OpenCode configuration files",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Generate opencode config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			svc := generator.NewService(d, "")
			return svc.GenerateConfig()
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "agents",
		Short: "Generate agents/*.md files",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			svc := generator.NewService(d, "")
			return svc.GenerateAgents()
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "commands",
		Short: "Generate commands/*.md files",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			svc := generator.NewService(d, "")
			return svc.GenerateCommands()
		},
	})
	return cmd
}

func newDailyCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Run full daily pipeline",
		Long: `Full daily pipeline:
  1. Backup snapshot
  2. Sync sources
  3. Discover models
  4. Audit models
  5. Generate config files
  6. Validate
  7. Heal if needed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			full, _ := cmd.Flags().GetBool("full")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			fmt.Println("=== Daily Pipeline ===")

			// 1. Backup
			fmt.Println("[1/9] Backup...")
			backupDir := filepath.Join(filepath.Dir(d.DBPath()), "backups")
			os.MkdirAll(backupDir, 0755)
			backupPath := filepath.Join(backupDir, fmt.Sprintf("opencode-kit-%s.tar.gz", time.Now().UTC().Format("2006-01-02T15-04-05")))
			if err := createBackup(d.DBPath(), backupPath); err != nil {
				fmt.Printf("  Warning: backup failed: %v\n", err)
			} else {
				fmt.Printf("  Backup: %s\n", backupPath)
				cleanupOldBackups(backupDir, 30*24*time.Hour)
			}

			// 2. Discover
			fmt.Println("[2/9] Discovering models...")
			dis := discover.NewService(discover.NewServiceParams{DB: d})
			if err := dis.Discover(cmd.Context()); err != nil {
				fmt.Printf("  Warning: discover: %v\n", err)
			}

			// 2a. Clean up stale preferences
			if cleaned, err := d.CleanupInvalidPreferences(); err == nil && cleaned > 0 {
				fmt.Printf("  Cleaned %d invalid preference values\n", cleaned)
			}
			if cleaned, err := d.CleanupProviderPrefs(); err == nil && cleaned > 0 {
				fmt.Printf("  Cleaned %d flat provider_* preferences\n", cleaned)
			}

			// 2b. Deduplicate providers with same base URL
			fmt.Println("[2b/9] Deduplicating providers with same base URL...")
			if err := dis.DeduplicateProviders(); err != nil {
				fmt.Printf("  Warning: dedup: %v\n", err)
			}

			// 2c. Live audit + fix drift (real /v1/models vs DB)
			fmt.Println("[2c/9] Live audit (real catalog vs DB) + auto-fix drift...")
			live := audit.NewLive(d, 4)
			fixes, err := live.FixAll(cmd.Context())
			if err != nil {
				fmt.Printf("  Warning: live fix: %v\n", err)
			} else {
				ph, ms, sk := 0, 0, 0
				for _, f := range fixes {
					if f.FetchError == "" {
						ph += f.PhantomFixed
						ms += f.MissingAdded
						sk += f.Skipped
					}
				}
				fmt.Printf("  Drift fixed: %d phantoms -> error, %d new untested, %d non-chat skipped\n",
					ph, ms, sk)
			}

			// 3. Audit
			fmt.Println("[3/9] Auditing models...")
			aud := audit.New(d, 5)
			if err := aud.Run(cmd.Context(), full); err != nil {
				fmt.Printf("  Warning: audit: %v\n", err)
			}

			// 3b. Activate untested models from known free providers
			fmt.Println("[3b/9] Activating untested free models...")
			if err := dis.ActivateUntestedFreeModels(); err != nil {
				fmt.Printf("  Warning: activate: %v\n", err)
			}

			// 4. Generate
			fmt.Println("[4/9] Generating configuration...")
			gen := generator.NewService(d, "")
			if err := gen.GenerateConfig(); err != nil {
				fmt.Printf("  Warning: generate config: %v\n", err)
			}

			// 5. Stats
			fmt.Println("[5/9] Collecting stats...")
			stats, err := d.GetStats()
			if err == nil {
				fmt.Printf("  Active models: %d\n", stats["active"])
				fmt.Printf("  Error models:  %d\n", stats["error"])
				fmt.Printf("  Untested:      %d\n", stats["untested"])
				fmt.Printf("  Providers:     %d\n", stats["providers_active"])
			}

			fmt.Println("[6/9] Profiling models...")
			prof := profile.New(d)
			if err := prof.ProfileAll(cmd.Context(), false); err != nil {
				fmt.Printf("  Warning: profile: %v\n", err)
			}

			fmt.Println("[7/9] Reassigning routing...")
			router := routing.New(d)
			if err := router.ReassignAll(cmd.Context(), false); err != nil {
				fmt.Printf("  Warning: route: %v\n", err)
			}

			fmt.Println("[8/9] Running auto-healing...")
			healer := heal.New(d)
			if report, err := healer.Run(cmd.Context()); err != nil {
				fmt.Printf("  Warning: heal: %v\n", err)
			} else if report.IssuesFound > 0 {
				fmt.Printf("  Healing: %d issues found, %d fixed\n", report.IssuesFound, report.IssuesFixed)
			}

			fmt.Println("[9/9] Compressing session observations...")
			fragments, _ := d.ListConfigFragments(50)
			obs := make([]compress.Observation, 0, len(fragments))
			step := 1
			for _, f := range fragments {
				obs = append(obs, compress.Observation{
					Source: f.Source, Step: step, Message: f.Content, Important: true,
				})
				step++
			}
			c := compress.NewWithDB(d, 12)
			if out := c.Compress(obs); out != "" {
				fmt.Printf("  Compressed: %d observation(s) -> fragment\n", len(obs))
			}

			fmt.Println("Daily pipeline complete.")
			return nil
		},
	}
	cmd.Flags().Bool("full", false, "Full re-audit of all models")
	return cmd
}

func newStatusCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show system status",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			stats, err := d.GetStats()
			if err != nil {
				return err
			}

			fmt.Println("=== opencode-kit Status ===")
			fmt.Printf("DB: %s\n", d.DBPath())
			fmt.Println()
			fmt.Println("Models by status:")
			for _, s := range []string{"active", "error", "untested", "deprecated", "paid"} {
				if c, ok := stats[s]; ok {
					fmt.Printf("  %s: %d\n", s, c)
				}
			}
			if p, ok := stats["providers_active"]; ok {
				fmt.Printf("\nActive providers: %d\n", p)
			}

			providers, err := d.ListProviders()
			if err == nil && len(providers) > 0 {
				fmt.Println("\nProviders:")
				for _, p := range providers {
					fmt.Printf("  %s (status: %s)\n", p.ID, p.Status)
				}
			}
			return nil
		},
	}
}

func newQueryCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "query",
		Short: "Run SQL query against DB",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			query := strings.Join(args, " ")
			rows, err := d.Query(query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			cols, _ := rows.Columns()
			fmt.Println(strings.Join(cols, " | "))
			fmt.Println(strings.Repeat("-", len(cols)*12))

			var rowVals []any = make([]any, len(cols))
			rowPtrs := make([]any, len(cols))
			for i := range rowVals {
				rowPtrs[i] = &rowVals[i]
			}

			for rows.Next() {
				if err := rows.Scan(rowPtrs...); err != nil {
					return err
				}
				strVals := make([]string, len(cols))
				for i, v := range rowVals {
					if v == nil {
						strVals[i] = "NULL"
					} else {
						strVals[i] = fmt.Sprintf("%v", v)
					}
				}
				fmt.Println(strings.Join(strVals, " | "))
			}
			return nil
		},
	}
}

func newProvidersCmd(dbPath *string) *cobra.Command {
	return newProvidersCmdImpl(dbPath)
}

func newModelsCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage models",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List models",
		RunE: func(cmd *cobra.Command, args []string) error {
			paid, _ := cmd.Flags().GetBool("paid")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			var opts []db.ModelFilter
			if !paid {
				opts = append(opts, db.StatusActive())
				opts = append(opts, db.Tier("free"))
			}
			models, err := d.ListModels(opts...)
			if err != nil {
				return err
			}
			if len(models) == 0 && !paid {
				// If no active models, show untested ones
				allModels, _ := d.ListModels()
				models = allModels
			}
			fmt.Printf("%-35s %-15s %-8s %-12s %s\n", "Model", "Provider", "Context", "FC", "Status")
			fmt.Println(strings.Repeat("-", 85))
			for _, m := range models {
				fc := "✓"
				if !m.FunctionCalling {
					fc = "-"
				}
				ctx := fmt.Sprintf("%dK", m.ContextWindow/1000)
				fmt.Printf("%-35s %-15s %-8s %-12s %s\n", m.ID, m.ProviderID, ctx, fc, m.Status)
			}
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "search",
		Short: "Search models",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			models, err := d.SearchModels(args[0])
			if err != nil {
				return err
			}
			for _, m := range models {
				fmt.Printf("%s/%s: %s (ctx: %d, FC: %v)\n", m.ProviderID, m.ID, m.Status, m.ContextWindow, m.FunctionCalling)
			}
			return nil
		},
	})
	cmd.AddCommand(listCmd)
	listCmd.Flags().Bool("paid", false, "Include paid models")
	return cmd
}

func newSyncCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Bidirectional sync between DB and config file",
		Long: `Bidirectional sync: imports changes from opencode config into DB,
then exports DB back to opencode config. Keeps both in sync.

Use after manually editing the config file, or after running
discover/audit/heal to push results to the config file.`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "bidirectional",
		Short: "Sync both directions (import → export)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			configPath := findConfigPath(d)
			svc := sync.New(d)

			fmt.Printf("Syncing %s → DB...\n", opencodeConfigName())
			inDiff, err := svc.ImportFromOpenCodeConfig(configPath)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			fmt.Printf("  New providers: %d, new models: %d, new agents: %d, new commands: %d\n",
				len(inDiff.AddedProviders), len(inDiff.AddedModels), len(inDiff.AddedAgents), len(inDiff.AddedCommands))

			fmt.Printf("Syncing DB → %s...\n", opencodeConfigName())
			if err := svc.ExportToOpenCodeConfig(configPath); err != nil {
				return fmt.Errorf("export: %w", err)
			}
			fmt.Printf("  %s updated.\n", opencodeConfigName())

			fmt.Println("Sync complete.")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "import",
		Short: "Import config file → DB only",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			svc := sync.New(d)
			configPath := findConfigPath(d)
			diff, err := svc.ImportFromOpenCodeConfig(configPath)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			fmt.Printf("Import complete. New providers: %d, new models: %d, new agents: %d, new commands: %d\n",
				len(diff.AddedProviders), len(diff.AddedModels), len(diff.AddedAgents), len(diff.AddedCommands))
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export DB → config file only",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			svc := sync.New(d)
			configPath := findConfigPath(d)
			if err := svc.ExportToOpenCodeConfig(configPath); err != nil {
				return fmt.Errorf("export: %w", err)
			}
			fmt.Printf("Export complete: %s updated.\n", opencodeConfigName())
			return nil
		},
	})
	return cmd
}

func findConfigPath(d *db.DB) string {
	configPath := OpenCodeConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configName := "opencode.json"
		if _, err := os.Stat(filepath.Join(filepath.Dir(d.DBPath()), "opencode.jsonc")); err == nil {
			configName = "opencode.jsonc"
		}
		configPath = filepath.Join(filepath.Dir(d.DBPath()), configName)
	}
	return configPath
}

func newSourcesCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage external sources",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List external sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			sources, err := d.ListSources()
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				fmt.Println("No sources configured")
				return nil
			}
			for _, s := range sources {
				fmt.Printf("%s: %s (%s)\n", s.ID, s.RemoteURL, s.Status)
			}
			return nil
		},
	})
	return cmd
}

func newProfileCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Profile all active models",
		RunE: func(cmd *cobra.Command, args []string) error {
			full, _ := cmd.Flags().GetBool("full")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			return profile.New(d).ProfileAll(cmd.Context(), full)
		},
	}
	cmd.Flags().Bool("full", false, "Full detailed profiling")
	return cmd
}

func newRouteCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Show or reassign model routing",
		RunE: func(cmd *cobra.Command, args []string) error {
			task, _ := cmd.Flags().GetString("task")
			reassign, _ := cmd.Flags().GetBool("reassign")
			shadow, _ := cmd.Flags().GetBool("shadow")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			r := routing.New(d)
			if reassign {
				return r.ReassignAll(cmd.Context(), shadow)
			}
			if task != "" {
				budget, _ := d.GetBudget()
				if budget == nil {
					budget = &models.BudgetConfig{ID: "default"}
				}
				rule, err := r.SelectBestModel(task, *budget, shadow)
				if err != nil {
					return err
				}
				fmt.Printf("Best model for %s: %s\n", task, rule.CurrentModelID)
				if rule.FallbackIDs != "" {
					var fallbacks []string
					if err := json.Unmarshal([]byte(rule.FallbackIDs), &fallbacks); err == nil && len(fallbacks) > 0 {
						fmt.Printf("Fallback chain: %s\n", strings.Join(fallbacks, " -> "))
					}
				}
				return nil
			}
			rules, err := d.ListRoutingRules()
			if err != nil {
				return err
			}
			for _, rule := range rules {
				fmt.Printf("%s: %s\n", rule.TaskKey, rule.CurrentModelID)
			}
			return nil
		},
	}
	cmd.AddCommand(newRouteReportCmd(dbPath))
	cmd.Flags().String("task", "", "Task type (coding_complex, coding_fast, reasoning, vision, long_context, fastest)")
	cmd.Flags().Bool("reassign", false, "Reassign all routing rules")
	cmd.Flags().Bool("shadow", false, "Log routing decisions without writing to DB")
	return cmd
}

func newRouteReportCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show recent routing decisions",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			events, err := d.ListRoutingEvents(limit)
			if err != nil {
				return err
			}
			if len(events) == 0 {
				fmt.Println("No routing events found")
				return nil
			}
			fmt.Printf("%-4s %-14s %-24s %-7s %s\n", "ID", "Task", "Model", "Shadow", "Reason")
			fmt.Println(strings.Repeat("-", 80))
			for _, e := range events {
				shadow := "no"
				if e.Shadow {
					shadow = "yes"
				}
				fmt.Printf("%-4d %-14s %-24s %-7s %s | %s\n", e.ID, e.TaskKey, e.SelectedModel, shadow, e.Reason, routing.FormatCandidateSummary(e.Candidates))
			}
			return nil
		},
	}
	cmd.Flags().Int("limit", 20, "Number of routing events to show")
	return cmd
}

func newHealCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "heal",
		Short: "Run auto-healing checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()
			report, err := heal.New(d).Run(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Printf("Issues found: %d, fixed: %d\n", report.IssuesFound, report.IssuesFixed)
			for _, issue := range report.Issues {
				status := "FIXED"
				if !issue.Fixed {
					status = "WARN"
				}
				fmt.Printf("  [%s] [%s] %s\n", status, issue.Severity, issue.Message)
			}
			return nil
		},
	}
}

func createBackup(dbPath, backupPath string) error {
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("read db: %w", err)
	}

	hdr := &tar.Header{
		Name: filepath.Base(dbPath),
		Size: int64(len(data)),
		Mode: 0644,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}

	return os.WriteFile(backupPath, buf.Bytes(), 0644)
}

func cleanupOldBackups(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, info.Name()))
		}
	}
}
