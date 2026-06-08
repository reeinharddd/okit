package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

func newModelsCmdImpl(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage models (CRUD + list + search)",
		Long: `Manage models: add, update, remove, list, search, info.
Auto-syncs to opencode config on every change.`,
	}

	cmd.AddCommand(newModelListCmd(dbPath))
	cmd.AddCommand(newModelSearchCmd(dbPath))
	cmd.AddCommand(newModelAddCmd(dbPath))
	cmd.AddCommand(newModelUpdateCmd(dbPath))
	cmd.AddCommand(newModelRemoveCmd(dbPath))
	cmd.AddCommand(newModelInfoCmd(dbPath))

	// Shared flags for add/update
	addUpdateFlags := []struct {
		name, short, desc string
	}{
		{"id", "", "Model ID (e.g. provider/model)"},
		{"display-name", "", "Display name (catalog raw ID)"},
		{"description", "", "Model description"},
		{"family", "", "Model family"},
		{"release-date", "", "Release date"},
		{"aliases", "", "Aliases (JSON array)"},
		{"deprecation", "", "Deprecation info (JSON)"},
		{"modalities-input", "", "Input modalities (JSON array)"},
		{"modalities-output", "", "Output modalities (JSON array)"},
		{"tags", "", "Tags (JSON array)"},
		{"tier", "", "Tier: free, paid, unknown"},
		{"status", "", "Status: active, error, deprecated, untested"},
		{"error-message", "", "Error message"},
	}
	for _, f := range addUpdateFlags {
		cmd.PersistentFlags().String(f.name, "", f.desc)
	}

	cmd.PersistentFlags().Int("context", 0, "Context window size")
	cmd.PersistentFlags().Int("max-output", 0, "Max output tokens")
	cmd.PersistentFlags().Float64("pricing-prompt", 0, "Prompt price per token")
	cmd.PersistentFlags().Float64("pricing-completion", 0, "Completion price per token")
	cmd.PersistentFlags().Float64("pricing-cache-read", 0, "Cache read price per token")
	cmd.PersistentFlags().Float64("pricing-cache-write", 0, "Cache write price per token")
	cmd.PersistentFlags().Float64("default-temperature", 0, "Default model temperature")

	boolFlags := []struct{ name, desc string }{
		{"function-calling", "Supports function calling"},
		{"vision", "Supports vision/image input"},
		{"reasoning", "Supports reasoning"},
		{"audio", "Supports audio"},
		{"ocr", "Supports OCR"},
		{"fine-tuning", "Supports fine-tuning"},
		{"classification", "Supports classification"},
		{"moderation", "Supports moderation"},
		{"streaming", "Supports streaming"},
		{"structured-outputs", "Supports structured outputs"},
		{"experimental", "Experimental model"},
	}
	for _, f := range boolFlags {
		cmd.PersistentFlags().Bool(f.name, false, f.desc)
	}

	cmd.PersistentFlags().String("provider-id", "", "Provider ID")
	cmd.PersistentFlags().String("interleaved", "", "Interleaved reasoning (JSON: true or {\"field\":\"reasoning_content\"})")
	cmd.PersistentFlags().Int("created-timestamp", 0, "Model creation timestamp")
	cmd.PersistentFlags().String("owned-by", "", "Model owner")

	return cmd
}

func newModelListCmd(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
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
				all, _ := d.ListModels()
				models = all
			}
			fmt.Printf("%-35s %-15s %-8s %-12s %-6s %-6s %s\n", "Model", "Provider", "Context", "FC", "Vis", "Reas", "Status")
			fmt.Println(strings.Repeat("-", 95))
			for _, m := range models {
				fc := "✓"
				if !m.FunctionCalling {
					fc = "-"
				}
				vis := "✓"
				if !m.Vision {
					vis = "-"
				}
				reas := "✓"
				if !m.Reasoning {
					reas = "-"
				}
				ctx := fmt.Sprintf("%dK", m.ContextWindow/1000)
				fmt.Printf("%-35s %-15s %-8s %-12s %-6s %-6s %s\n", m.ID, m.ProviderID, ctx, fc, vis, reas, m.Status)
			}
			return nil
		},
	}
	cmd.Flags().Bool("paid", false, "Include paid models")
	return cmd
}

func newModelSearchCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "search",
		Short: "Search models by keyword",
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
				fmt.Printf("%s/%s: %s (ctx: %d, FC: %v, Vis: %v, Reas: %v, Audio: %v, OCR: %v)\n",
					m.ProviderID, m.ID, m.Status, m.ContextWindow,
					m.FunctionCalling, m.Vision, m.Reasoning, m.Audio, m.OCR)
			}
			return nil
		},
	}
}

func newModelAddCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a custom model",
		Long: `Add a model to the database.
Auto-syncs to opencode config.

Example:
  okit models add --id my-provider/my-model --provider-id my-provider --context 128000 --function-calling`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			provID, _ := cmd.Flags().GetString("provider-id")
			if id == "" || provID == "" {
				return fmt.Errorf("required: --id, --provider-id")
			}

			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			m := &models.Model{
				ID:               id,
				ProviderID:       provID,
				Source:           "manual",
				Status:           "untested",
				Streaming:        true,
				Tier:             "unknown",
			}

			applyModelFlags(cmd, m)

			if err := d.UpsertModel(m); err != nil {
				return fmt.Errorf("db insert: %w", err)
			}
			fmt.Printf("Model %s added to database.\n", id)

			return syncConfig(d)
		},
	}
}

func newModelUpdateCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update an existing model",
		Long: `Update model fields. Only provided flags are changed.
Auto-syncs to opencode config.

Example:
  okit models update --id groq/llama-3-70b --reasoning`,
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

			existing, err := d.GetModel(id)
			if err != nil {
				return fmt.Errorf("model %q not found: %w", id, err)
			}

			if applyModelFlags(cmd, existing) == 0 {
				return fmt.Errorf("no fields to update (specify at least one flag)")
			}

			if err := d.UpsertModel(existing); err != nil {
				return fmt.Errorf("db update: %w", err)
			}
			fmt.Printf("Model %s updated in database.\n", id)

			return syncConfig(d)
		},
	}
}

func newModelRemoveCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove a model",
		Long: `Remove a model from the database.
Auto-syncs to opencode config.

Example:
  okit models remove --id groq/llama-3-70b`,
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

			if err := d.DeleteModel(id); err != nil {
				return fmt.Errorf("db delete: %w", err)
			}
			fmt.Printf("Model %s removed from database.\n", id)

			return syncConfig(d)
		},
	}
}

func newModelInfoCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show detailed model info",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB(dbPath)
			if err != nil {
				return err
			}
			defer d.Close()

			m, err := d.GetModel(args[0])
			if err != nil {
				return fmt.Errorf("model %q not found: %w", args[0], err)
			}

			fmt.Printf("ID:               %s\n", m.ID)
			fmt.Printf("Provider:         %s\n", m.ProviderID)
			fmt.Printf("Display Name:     %s\n", m.DisplayName)
			fmt.Printf("Description:      %s\n", m.Description)
			fmt.Printf("Family:           %s\n", m.Family)
			fmt.Printf("Release Date:     %s\n", m.ReleaseDate)
			fmt.Printf("Context Window:   %d\n", m.ContextWindow)
			fmt.Printf("Max Output:       %d\n", m.MaxOutput)
			fmt.Printf("Tier:             %s\n", m.Tier)
			fmt.Printf("Status:           %s\n", m.Status)
			fmt.Printf("Source:           %s\n", m.Source)
			fmt.Println()
			fmt.Println("Capabilities:")
			fmt.Printf("  Function Calling: %v\n", m.FunctionCalling)
			fmt.Printf("  Vision:           %v\n", m.Vision)
			fmt.Printf("  Reasoning:        %v\n", m.Reasoning)
			fmt.Printf("  Audio:            %v\n", m.Audio)
			fmt.Printf("  OCR:              %v\n", m.OCR)
			fmt.Printf("  Streaming:        %v\n", m.Streaming)
			fmt.Printf("  Structured Out:   %v\n", m.StructuredOutput)
			fmt.Printf("  Experimental:     %v\n", m.Experimental)
			fmt.Println()
			fmt.Println("Pricing:")
			fmt.Printf("  Prompt:           %.10f\n", m.PricingPrompt)
			fmt.Printf("  Completion:       %.10f\n", m.PricingCompletion)
			fmt.Printf("  Cache Read:       %.10f\n", m.PricingCacheRead)
			fmt.Printf("  Cache Write:      %.10f\n", m.PricingCacheWrite)
			fmt.Printf("  Default Temp:     %.2f\n", m.DefaultTemp)
			fmt.Println()
			fmt.Println("Extra Capabilities:")
			fmt.Printf("  Fine Tuning:      %v\n", m.FineTuning)
			fmt.Printf("  Classification:   %v\n", m.Classification)
			fmt.Printf("  Moderation:       %v\n", m.Moderation)
			fmt.Println()
			fmt.Printf("Aliases:          %s\n", m.Aliases)
			fmt.Printf("Deprecation:      %s\n", m.Deprecation)
			fmt.Printf("Interleaved:      %s\n", m.Interleaved)
			fmt.Printf("Modalities In:    %s\n", m.ModalitiesInput)
			fmt.Printf("Modalities Out:   %s\n", m.ModalitiesOutput)
			fmt.Printf("Created:          %d\n", m.CreatedTimestamp)
			fmt.Printf("Owned By:         %s\n", m.OwnedBy)
			fmt.Printf("Tags:             %s\n", m.Tags)
			fmt.Printf("Error Message:    %s\n", m.ErrorMessage)
			return nil
		},
	}
}

func applyModelFlags(cmd *cobra.Command, m *models.Model) int {
	changed := 0

	if v, _ := cmd.Flags().GetString("display-name"); cmd.Flags().Changed("display-name") {
		m.DisplayName = v; changed++
	}
	if v, _ := cmd.Flags().GetString("description"); cmd.Flags().Changed("description") {
		m.Description = v; changed++
	}
	if v, _ := cmd.Flags().GetString("family"); cmd.Flags().Changed("family") {
		m.Family = v; changed++
	}
	if v, _ := cmd.Flags().GetString("release-date"); cmd.Flags().Changed("release-date") {
		m.ReleaseDate = v; changed++
	}
	if v, _ := cmd.Flags().GetString("aliases"); cmd.Flags().Changed("aliases") {
		m.Aliases = v; changed++
	}
	if v, _ := cmd.Flags().GetString("deprecation"); cmd.Flags().Changed("deprecation") {
		m.Deprecation = v; changed++
	}
	if v, _ := cmd.Flags().GetString("modalities-input"); cmd.Flags().Changed("modalities-input") {
		m.ModalitiesInput = v; changed++
	}
	if v, _ := cmd.Flags().GetString("modalities-output"); cmd.Flags().Changed("modalities-output") {
		m.ModalitiesOutput = v; changed++
	}
	if v, _ := cmd.Flags().GetString("tags"); cmd.Flags().Changed("tags") {
		m.Tags = v; changed++
	}
	if v, _ := cmd.Flags().GetString("tier"); cmd.Flags().Changed("tier") {
		m.Tier = v; changed++
	}
	if v, _ := cmd.Flags().GetString("status"); cmd.Flags().Changed("status") {
		m.Status = v; changed++
	}
	if v, _ := cmd.Flags().GetString("error-message"); cmd.Flags().Changed("error-message") {
		m.ErrorMessage = v; changed++
	}
	if v, _ := cmd.Flags().GetString("provider-id"); cmd.Flags().Changed("provider-id") {
		m.ProviderID = v; changed++
	}
	if v, _ := cmd.Flags().GetInt("context"); cmd.Flags().Changed("context") {
		m.ContextWindow = v; changed++
	}
	if v, _ := cmd.Flags().GetInt("max-output"); cmd.Flags().Changed("max-output") {
		m.MaxOutput = v; changed++
	}
	if v, _ := cmd.Flags().GetFloat64("pricing-prompt"); cmd.Flags().Changed("pricing-prompt") {
		m.PricingPrompt = v; changed++
	}
	if v, _ := cmd.Flags().GetFloat64("pricing-completion"); cmd.Flags().Changed("pricing-completion") {
		m.PricingCompletion = v; changed++
	}
	if v, _ := cmd.Flags().GetFloat64("pricing-cache-read"); cmd.Flags().Changed("pricing-cache-read") {
		m.PricingCacheRead = v; changed++
	}
	if v, _ := cmd.Flags().GetFloat64("pricing-cache-write"); cmd.Flags().Changed("pricing-cache-write") {
		m.PricingCacheWrite = v; changed++
	}
	if v, _ := cmd.Flags().GetFloat64("default-temperature"); cmd.Flags().Changed("default-temperature") {
		m.DefaultTemp = v; changed++
	}
	if v, _ := cmd.Flags().GetInt("created-timestamp"); cmd.Flags().Changed("created-timestamp") {
		m.CreatedTimestamp = int64(v); changed++
	}
	if v, _ := cmd.Flags().GetString("owned-by"); cmd.Flags().Changed("owned-by") {
		m.OwnedBy = v; changed++
	}
	if v, _ := cmd.Flags().GetString("interleaved"); cmd.Flags().Changed("interleaved") {
		m.Interleaved = v; changed++
	}

	extraBoolFlags := []struct {
		name string
		field *bool
	}{
		{"fine-tuning", &m.FineTuning},
		{"classification", &m.Classification},
		{"moderation", &m.Moderation},
	}
	for _, f := range extraBoolFlags {
		if cmd.Flags().Changed(f.name) {
			v, _ := cmd.Flags().GetBool(f.name)
			*f.field = v
			changed++
		}
	}

	boolFlags := []struct {
		name string
		field *bool
	}{
		{"function-calling", &m.FunctionCalling},
		{"vision", &m.Vision},
		{"reasoning", &m.Reasoning},
		{"audio", &m.Audio},
		{"ocr", &m.OCR},
		{"streaming", &m.Streaming},
		{"structured-outputs", &m.StructuredOutput},
		{"experimental", &m.Experimental},
	}
	for _, f := range boolFlags {
		if cmd.Flags().Changed(f.name) {
			v, _ := cmd.Flags().GetBool(f.name)
			*f.field = v
			changed++
		}
	}

	return changed
}
