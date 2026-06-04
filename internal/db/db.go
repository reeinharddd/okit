package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/reeinharddd/okit/internal/config"
	"github.com/reeinharddd/okit/pkg/models"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
	Path string
}

var _ DBInterface = (*DB)(nil)

func DefaultPath() string {
	base := config.ConfigDir()
	os.MkdirAll(base, 0755)
	return filepath.Join(base, "opencode-kit.db")
}

func Open(path string) (*DB, error) {
	if path == "" {
		path = DefaultPath()
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	d := &DB{DB: db, Path: path}
	if err := d.SeedDefaults(); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.DB.Close()
}

func (d *DB) DBPath() string {
	return d.Path
}

func (d *DB) Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (d *DB) ExecLog(syncType, status, message string, _ time.Duration) error {
	_, err := d.Exec(
		`INSERT INTO sync_log (phase, status, details) VALUES (?, ?, ?)`,
		syncType, status, message,
	)
	return err
}

func Migrate(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

var seedProviders = []models.Provider{
	{ID: "groq", Name: "Groq", BaseURL: "https://api.groq.com/openai/v1", CatalogURL: "https://api.groq.com/openai/v1/models", KeyEnv: "GROQ_API_KEY", Source: "seed", Status: "active", Priority: 10},
	{ID: "mistral", Name: "Mistral", BaseURL: "https://api.mistral.ai/v1", CatalogURL: "https://api.mistral.ai/v1/models", KeyEnv: "MISTRAL_API_KEY", Source: "seed", Status: "active", Priority: 20},
	{ID: "nvidia", Name: "NVIDIA", BaseURL: "https://integrate.api.nvidia.com/v1", CatalogURL: "https://integrate.api.nvidia.com/v1/models", KeyEnv: "NVIDIA_API_KEY", Source: "seed", Status: "active", Priority: 30},
	{ID: "cerebras", Name: "Cerebras", BaseURL: "https://api.cerebras.ai/v1", CatalogURL: "https://api.cerebras.ai/public/v1/models", KeyEnv: "CEREBRAS_API_KEY", Source: "seed", Status: "active", Priority: 40},
	{ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", CatalogURL: "https://openrouter.ai/api/v1/models", KeyEnv: "OPENROUTER_API_KEY", Source: "seed", Status: "active", Priority: 50},
	{ID: "github-models", Name: "GitHub Models", BaseURL: "https://models.github.ai/inference", CatalogURL: "https://models.github.ai/catalog/models", KeyEnv: "GITHUB_TOKEN", Source: "seed", Status: "active", Priority: 60},
	{ID: "opencode-zen", Name: "OpenCode Zen", BaseURL: "https://opencode.ai/zen/v1", CatalogURL: "https://opencode.ai/zen/v1/models", KeyEnv: "OPENCODE_ZEN_API_KEY", Source: "seed", Status: "active", Priority: 70, IsFree: true},
	{ID: "github-copilot", Name: "GitHub Copilot", BaseURL: "https://api.githubcopilot.com", CatalogURL: "https://api.githubcopilot.com/models", KeyEnv: "GITHUB_TOKEN", Source: "seed", Status: "active", Priority: 80},
}

func (d *DB) SeedDefaults() error {
	_, err := d.Exec(`INSERT OR IGNORE INTO budget_config (id, daily_global_usd, preferred_tier) VALUES ('default', 0.50, 'free_only')`)
	if err != nil {
		return fmt.Errorf("seed budget: %w", err)
	}
	_, err = d.Exec(`INSERT OR IGNORE INTO routing_rules (task_key, description, min_context, needs_fc, needs_vision, max_cost_per_call, current_model_id, last_assigned) VALUES
		('coding_complex', 'Complex coding tasks with function calling', 100000, 1, 0, 0, '', 0),
		('coding_fast', 'Fast coding with function calling', 50000, 1, 0, 0, '', 0),
		('reasoning', 'Deep reasoning and analysis', 100000, 0, 0, 0, '', 0),
		('vision', 'Vision and image understanding', 100000, 0, 1, 0, '', 0),
		('long_context', 'Long context research and analysis', 500000, 0, 0, 0, '', 0),
		('fastest', 'Simple tasks, maximum speed', 0, 0, 0, 0, '', 0)`)
	if err != nil {
		return fmt.Errorf("seed routing rules: %w", err)
	}
	for _, p := range seedProviders {
		if err := d.UpsertProvider(&p); err != nil {
			return fmt.Errorf("seed provider %s: %w", p.ID, err)
		}
	}
	return nil
}
