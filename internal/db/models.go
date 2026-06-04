package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/reeinharddd/okit/pkg/models"
)

func (d *DB) UpsertProvider(p *models.Provider) error {
	_, err := d.Exec(`INSERT INTO providers (id, name, api_base, catalog_url, key_env, is_free, source, status, priority, last_synced)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, api_base=excluded.api_base, catalog_url=excluded.catalog_url,
		key_env=excluded.key_env, is_free=excluded.is_free,
		source=excluded.source, status=excluded.status,
		priority=excluded.priority, last_synced=excluded.last_synced`,
		p.ID, p.Name, p.BaseURL, p.CatalogURL, p.KeyEnv, boolToInt(p.IsFree),
		p.Source, p.Status, p.Priority, p.LastSynced)
	return err
}

func (d *DB) ListProviders() ([]models.Provider, error) {
	rows, err := d.Query(`SELECT id, name, COALESCE(api_base,''), COALESCE(catalog_url,''), COALESCE(key_env,''), COALESCE(is_free,0), COALESCE(source,'auto'), COALESCE(status,'active'), COALESCE(priority,99), COALESCE(last_synced,0) FROM providers ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Provider
	for rows.Next() {
		var p models.Provider
		var isFree int
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.CatalogURL, &p.KeyEnv, &isFree, &p.Source, &p.Status, &p.Priority, &p.LastSynced); err != nil {
			return nil, err
		}
		p.IsFree = isFree != 0
		out = append(out, p)
	}
	return out, nil
}

func (d *DB) GetProvider(id string) (*models.Provider, error) {
	var p models.Provider
	var isFree int
	err := d.QueryRow(`SELECT id, name, COALESCE(api_base,''), COALESCE(catalog_url,''), COALESCE(key_env,''), COALESCE(is_free,0), COALESCE(source,'auto'), COALESCE(status,'active'), COALESCE(priority,99), COALESCE(last_synced,0) FROM providers WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.BaseURL, &p.CatalogURL, &p.KeyEnv, &isFree, &p.Source, &p.Status, &p.Priority, &p.LastSynced)
	if err != nil {
		return nil, err
	}
	p.IsFree = isFree != 0
	return &p, nil
}

func (d *DB) DeleteProvider(id string) error {
	_, err := d.Exec(`DELETE FROM models WHERE provider_id=?`, id)
	if err != nil {
		return err
	}
	_, err = d.Exec(`DELETE FROM providers WHERE id=?`, id)
	return err
}

func (d *DB) UpsertModel(m *models.Model) error {
	_, err := d.Exec(`INSERT INTO models (id, provider_id, display_name, description, context_window, function_calling, vision, streaming, structured_outputs, latency_p50_ms, latency_p95_ms, tokens_per_sec, pricing_prompt, pricing_completion, pricing_cache_read, tier, status, error_message, tags, last_tested, test_count, fail_count, source, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
		display_name=excluded.display_name, description=excluded.description,
		context_window=excluded.context_window, function_calling=excluded.function_calling,
		vision=excluded.vision, streaming=excluded.streaming, structured_outputs=excluded.structured_outputs,
		latency_p50_ms=excluded.latency_p50_ms, latency_p95_ms=excluded.latency_p95_ms,
		tokens_per_sec=excluded.tokens_per_sec,
		pricing_prompt=excluded.pricing_prompt, pricing_completion=excluded.pricing_completion,
		pricing_cache_read=excluded.pricing_cache_read,
		tier=excluded.tier, status=excluded.status, error_message=excluded.error_message,
		tags=excluded.tags, last_tested=excluded.last_tested, test_count=excluded.test_count,
		fail_count=excluded.fail_count, source=excluded.source, updated_at=datetime('now')`,
		m.ID, m.ProviderID, m.DisplayName, m.Description, m.ContextWindow,
		boolToInt(m.FunctionCalling), boolToInt(m.Vision), boolToInt(m.Streaming), boolToInt(m.StructuredOutput),
		m.LatencyP50Ms, m.LatencyP95Ms, m.TokensPerSec,
		m.PricingPrompt, m.PricingCompletion, m.PricingCacheRead,
		m.Tier, m.Status, m.ErrorMessage, m.Tags, m.LastTested, m.TestCount, m.FailCount, m.Source)
	return err
}

func (d *DB) ListModelsByProvider(providerID string) ([]models.Model, error) {
	rows, err := d.Query(`SELECT id, provider_id, COALESCE(display_name,''), COALESCE(description,''), COALESCE(context_window,0), COALESCE(function_calling,0), COALESCE(vision,0), COALESCE(streaming,1), COALESCE(structured_outputs,0), COALESCE(latency_p50_ms,0), COALESCE(latency_p95_ms,0), COALESCE(tokens_per_sec,0), COALESCE(pricing_prompt,0), COALESCE(pricing_completion,0), COALESCE(pricing_cache_read,0), COALESCE(tier,'unknown'), COALESCE(status,'untested'), COALESCE(error_message,''), COALESCE(tags,''), COALESCE(last_tested,0), COALESCE(test_count,0), COALESCE(fail_count,0), COALESCE(source,'discovered') FROM models WHERE provider_id=? ORDER BY status, id`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

func (d *DB) ListModels(opts ...ModelFilter) ([]models.Model, error) {
	query := `SELECT id, provider_id, COALESCE(display_name,''), COALESCE(description,''), COALESCE(context_window,0), COALESCE(function_calling,0), COALESCE(vision,0), COALESCE(streaming,1), COALESCE(structured_outputs,0), COALESCE(latency_p50_ms,0), COALESCE(latency_p95_ms,0), COALESCE(tokens_per_sec,0), COALESCE(pricing_prompt,0), COALESCE(pricing_completion,0), COALESCE(pricing_cache_read,0), COALESCE(tier,'unknown'), COALESCE(status,'untested'), COALESCE(error_message,''), COALESCE(tags,''), COALESCE(last_tested,0), COALESCE(test_count,0), COALESCE(fail_count,0), COALESCE(source,'discovered') FROM models`
	var args []any
	var clauses []string
	for _, opt := range opts {
		s, a := opt()
		if s != "" {
			clauses = append(clauses, s)
			args = append(args, a...)
		}
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY provider_id, status, id"
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

type ModelFilter func() (string, []any)

func StatusActive() ModelFilter {
	return func() (string, []any) { return "status='active'", nil }
}
func StatusNot(string2 string) ModelFilter {
	return func() (string, []any) { return "status!=?", []any{string2} }
}
func HasFC() ModelFilter {
	return func() (string, []any) { return "function_calling=1", nil }
}
func MinContext(min int) ModelFilter {
	return func() (string, []any) { return "context_window>=?", []any{min} }
}
func Tier(tier string) ModelFilter {
	return func() (string, []any) { return "tier=?", []any{tier} }
}

func scanModels(rows *sql.Rows) ([]models.Model, error) {
	var out []models.Model
	for rows.Next() {
		var m models.Model
		var fc, vis, str, so int
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.DisplayName, &m.Description, &m.ContextWindow,
			&fc, &vis, &str, &so,
			&m.LatencyP50Ms, &m.LatencyP95Ms, &m.TokensPerSec,
			&m.PricingPrompt, &m.PricingCompletion, &m.PricingCacheRead,
			&m.Tier, &m.Status, &m.ErrorMessage, &m.Tags,
			&m.LastTested, &m.TestCount, &m.FailCount, &m.Source); err != nil {
			return nil, err
		}
		m.FunctionCalling = fc != 0
		m.Vision = vis != 0
		m.Streaming = str != 0
		m.StructuredOutput = so != 0
		out = append(out, m)
	}
	return out, nil
}

func (d *DB) GetModel(id string) (*models.Model, error) {
	var m models.Model
	var fc, vis, str, so int
	err := d.QueryRow(`SELECT id, provider_id, COALESCE(display_name,''), COALESCE(description,''), COALESCE(context_window,0), COALESCE(function_calling,0), COALESCE(vision,0), COALESCE(streaming,1), COALESCE(structured_outputs,0), COALESCE(latency_p50_ms,0), COALESCE(latency_p95_ms,0), COALESCE(tokens_per_sec,0), COALESCE(pricing_prompt,0), COALESCE(pricing_completion,0), COALESCE(pricing_cache_read,0), COALESCE(tier,'unknown'), COALESCE(status,'untested'), COALESCE(error_message,''), COALESCE(tags,''), COALESCE(last_tested,0), COALESCE(test_count,0), COALESCE(fail_count,0), COALESCE(source,'discovered') FROM models WHERE id=?`, id).
		Scan(&m.ID, &m.ProviderID, &m.DisplayName, &m.Description, &m.ContextWindow,
			&fc, &vis, &str, &so,
			&m.LatencyP50Ms, &m.LatencyP95Ms, &m.TokensPerSec,
			&m.PricingPrompt, &m.PricingCompletion, &m.PricingCacheRead,
			&m.Tier, &m.Status, &m.ErrorMessage, &m.Tags,
			&m.LastTested, &m.TestCount, &m.FailCount, &m.Source)
	if err != nil {
		return nil, err
	}
	m.FunctionCalling = fc != 0
	m.Vision = vis != 0
	m.Streaming = str != 0
	m.StructuredOutput = so != 0
	return &m, nil
}

func (d *DB) DeleteModel(id string) error {
	_, err := d.Exec(`DELETE FROM models WHERE id=?`, id)
	return err
}

func (d *DB) GetStats() (map[string]int, error) {
	stats := make(map[string]int)
	rows, err := d.Query(`SELECT status, COUNT(*) FROM models GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var c int
		if err := rows.Scan(&s, &c); err != nil {
			return nil, err
		}
		stats[s] = c
	}
	row := d.QueryRow(`SELECT COUNT(DISTINCT provider_id) FROM models WHERE status='active'`)
	var provs int
	if err := row.Scan(&provs); err == nil {
		stats["providers_active"] = provs
	}
	return stats, nil
}

func (d *DB) SearchModels(query string) ([]models.Model, error) {
	q := "%" + query + "%"
	rows, err := d.Query(`SELECT id, provider_id, COALESCE(display_name,''), COALESCE(description,''), COALESCE(context_window,0), COALESCE(function_calling,0), COALESCE(vision,0), COALESCE(streaming,1), COALESCE(structured_outputs,0), COALESCE(latency_p50_ms,0), COALESCE(latency_p95_ms,0), COALESCE(tokens_per_sec,0), COALESCE(pricing_prompt,0), COALESCE(pricing_completion,0), COALESCE(pricing_cache_read,0), COALESCE(tier,'unknown'), COALESCE(status,'untested'), COALESCE(error_message,''), COALESCE(tags,''), COALESCE(last_tested,0), COALESCE(test_count,0), COALESCE(fail_count,0), COALESCE(source,'discovered') FROM models WHERE id LIKE ? OR display_name LIKE ? OR description LIKE ? OR tags LIKE ? ORDER BY status, provider_id`, q, q, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func fmtProvider(p models.Provider) string {
	return fmt.Sprintf("%s (%s)", p.ID, p.Name)
}
