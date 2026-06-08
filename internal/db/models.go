package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/reeinharddd/okit/pkg/models"
)

func (d *DB) UpsertProvider(p *models.Provider) error {
	_, err := d.Exec(`INSERT INTO providers (id, name, api_base, catalog_url, key_env, is_free, enabled, source, status, priority, timeout_ms, header_timeout_ms, chunk_timeout_ms, enterprise_url, set_cache_key, api_package, env_list, last_synced)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, api_base=excluded.api_base, catalog_url=excluded.catalog_url,
		key_env=excluded.key_env, is_free=excluded.is_free, enabled=excluded.enabled,
		source=excluded.source, status=excluded.status,
		priority=excluded.priority,
		timeout_ms=excluded.timeout_ms, header_timeout_ms=excluded.header_timeout_ms,
		chunk_timeout_ms=excluded.chunk_timeout_ms, enterprise_url=excluded.enterprise_url,
		set_cache_key=excluded.set_cache_key, api_package=excluded.api_package,
		env_list=excluded.env_list, last_synced=excluded.last_synced`,
		p.ID, p.Name, p.BaseURL, p.CatalogURL, p.KeyEnv, boolToInt(p.IsFree),
		boolToInt(p.Enabled), p.Source, p.Status, p.Priority,
		p.TimeoutMs, p.HeaderTimeoutMs, p.ChunkTimeoutMs, p.EnterpriseURL,
		boolToInt(p.SetCacheKey), p.APIPackage, p.EnvList, p.LastSynced)
	return err
}

var providerCols = `id, name, COALESCE(api_base,''), COALESCE(catalog_url,''), COALESCE(key_env,''), COALESCE(is_free,0), COALESCE(enabled,1), COALESCE(source,'auto'), COALESCE(status,'active'), COALESCE(priority,99), COALESCE(timeout_ms,0), COALESCE(header_timeout_ms,0), COALESCE(chunk_timeout_ms,0), COALESCE(enterprise_url,''), COALESCE(set_cache_key,0), COALESCE(api_package,''), COALESCE(env_list,''), COALESCE(last_synced,0)`

func scanProvider(scanner interface{ Scan(dest ...interface{}) error }) (models.Provider, error) {
	var p models.Provider
	var isFree, enabled, setCache int
	err := scanner.Scan(&p.ID, &p.Name, &p.BaseURL, &p.CatalogURL, &p.KeyEnv,
		&isFree, &enabled, &p.Source, &p.Status, &p.Priority,
		&p.TimeoutMs, &p.HeaderTimeoutMs, &p.ChunkTimeoutMs, &p.EnterpriseURL,
		&setCache, &p.APIPackage, &p.EnvList, &p.LastSynced)
	if err != nil {
		return p, err
	}
	p.IsFree = isFree != 0
	p.Enabled = enabled != 0
	p.SetCacheKey = setCache != 0
	return p, nil
}

func (d *DB) ListProviders() ([]models.Provider, error) {
	rows, err := d.Query(`SELECT ` + providerCols + ` FROM providers ORDER BY priority`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (d *DB) GetProvider(id string) (*models.Provider, error) {
	p, err := scanProvider(d.QueryRow(`SELECT `+providerCols+` FROM providers WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
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
	_, err := d.Exec(`INSERT INTO models (id, provider_id, display_name, description, context_window, max_output, function_calling, vision, reasoning, audio, ocr, fine_tuning, classification, moderation, streaming, structured_outputs, latency_p50_ms, latency_p95_ms, tokens_per_sec, pricing_prompt, pricing_completion, pricing_cache_read, pricing_cache_write, default_temperature, tier, status, error_message, tags, aliases, family, release_date, deprecation, interleaved, experimental, modalities_input, modalities_output, created_timestamp, owned_by, last_tested, test_count, fail_count, source, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
		display_name=excluded.display_name, description=excluded.description,
		context_window=excluded.context_window, max_output=excluded.max_output,
		function_calling=excluded.function_calling, vision=excluded.vision,
		reasoning=excluded.reasoning, audio=excluded.audio, ocr=excluded.ocr,
		fine_tuning=excluded.fine_tuning, classification=excluded.classification,
		moderation=excluded.moderation,
		streaming=excluded.streaming, structured_outputs=excluded.structured_outputs,
		latency_p50_ms=excluded.latency_p50_ms, latency_p95_ms=excluded.latency_p95_ms,
		tokens_per_sec=excluded.tokens_per_sec,
		pricing_prompt=excluded.pricing_prompt, pricing_completion=excluded.pricing_completion,
		pricing_cache_read=excluded.pricing_cache_read, pricing_cache_write=excluded.pricing_cache_write,
		default_temperature=excluded.default_temperature,
		tier=excluded.tier, status=excluded.status, error_message=excluded.error_message,
		tags=excluded.tags, aliases=excluded.aliases, family=excluded.family,
		release_date=excluded.release_date, deprecation=excluded.deprecation,
		interleaved=excluded.interleaved,
		experimental=excluded.experimental, modalities_input=excluded.modalities_input,
		modalities_output=excluded.modalities_output,
		created_timestamp=excluded.created_timestamp, owned_by=excluded.owned_by,
		last_tested=excluded.last_tested, test_count=excluded.test_count,
		fail_count=excluded.fail_count, source=excluded.source, updated_at=datetime('now')`,
		m.ID, m.ProviderID, m.DisplayName, m.Description, m.ContextWindow, m.MaxOutput,
		boolToInt(m.FunctionCalling), boolToInt(m.Vision), boolToInt(m.Reasoning),
		boolToInt(m.Audio), boolToInt(m.OCR),
		boolToInt(m.FineTuning), boolToInt(m.Classification), boolToInt(m.Moderation),
		boolToInt(m.Streaming), boolToInt(m.StructuredOutput),
		m.LatencyP50Ms, m.LatencyP95Ms, m.TokensPerSec,
		m.PricingPrompt, m.PricingCompletion, m.PricingCacheRead, m.PricingCacheWrite,
		m.DefaultTemp,
		m.Tier, m.Status, m.ErrorMessage, m.Tags, m.Aliases, m.Family, m.ReleaseDate,
		m.Deprecation, m.Interleaved, boolToInt(m.Experimental),
		m.ModalitiesInput, m.ModalitiesOutput,
		m.CreatedTimestamp, m.OwnedBy,
		m.LastTested, m.TestCount, m.FailCount, m.Source)
	return err
}

var modelCols = `id, provider_id, COALESCE(display_name,''), COALESCE(description,''), COALESCE(context_window,0), COALESCE(max_output,0), COALESCE(function_calling,0), COALESCE(vision,0), COALESCE(reasoning,0), COALESCE(audio,0), COALESCE(ocr,0), COALESCE(fine_tuning,0), COALESCE(classification,0), COALESCE(moderation,0), COALESCE(streaming,1), COALESCE(structured_outputs,0), COALESCE(latency_p50_ms,0), COALESCE(latency_p95_ms,0), COALESCE(tokens_per_sec,0), COALESCE(pricing_prompt,0), COALESCE(pricing_completion,0), COALESCE(pricing_cache_read,0), COALESCE(pricing_cache_write,0), COALESCE(default_temperature,0), COALESCE(tier,'unknown'), COALESCE(status,'untested'), COALESCE(error_message,''), COALESCE(tags,''), COALESCE(aliases,''), COALESCE(family,''), COALESCE(release_date,''), COALESCE(deprecation,''), COALESCE(interleaved,''), COALESCE(experimental,0), COALESCE(modalities_input,''), COALESCE(modalities_output,''), COALESCE(created_timestamp,0), COALESCE(owned_by,''), COALESCE(last_tested,0), COALESCE(test_count,0), COALESCE(fail_count,0), COALESCE(source,'discovered')`

func (d *DB) ListModelsByProvider(providerID string) ([]models.Model, error) {
	rows, err := d.Query(`SELECT `+modelCols+` FROM models WHERE provider_id=? ORDER BY status, id`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

func (d *DB) ListModels(opts ...ModelFilter) ([]models.Model, error) {
	query := `SELECT ` + modelCols + ` FROM models`
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
		var fc, vis, reas, aud, ocr, ft, cls, mod, str, so, exp int
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.DisplayName, &m.Description, &m.ContextWindow,
			&m.MaxOutput, &fc, &vis, &reas, &aud, &ocr, &ft, &cls, &mod,
			&str, &so,
			&m.LatencyP50Ms, &m.LatencyP95Ms, &m.TokensPerSec,
			&m.PricingPrompt, &m.PricingCompletion, &m.PricingCacheRead, &m.PricingCacheWrite,
			&m.DefaultTemp,
			&m.Tier, &m.Status, &m.ErrorMessage, &m.Tags, &m.Aliases, &m.Family,
			&m.ReleaseDate, &m.Deprecation, &m.Interleaved, &exp,
			&m.ModalitiesInput, &m.ModalitiesOutput,
			&m.CreatedTimestamp, &m.OwnedBy,
			&m.LastTested, &m.TestCount, &m.FailCount, &m.Source); err != nil {
			return nil, err
		}
		m.FunctionCalling = fc != 0
		m.Vision = vis != 0
		m.Reasoning = reas != 0
		m.Audio = aud != 0
		m.OCR = ocr != 0
		m.FineTuning = ft != 0
		m.Classification = cls != 0
		m.Moderation = mod != 0
		m.Streaming = str != 0
		m.StructuredOutput = so != 0
		m.Experimental = exp != 0
		out = append(out, m)
	}
	return out, nil
}

func (d *DB) GetModel(id string) (*models.Model, error) {
	rows, err := d.Query(`SELECT `+modelCols+` FROM models WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models, err := scanModels(rows)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("model %q not found", id)
	}
	return &models[0], nil
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
	rows, err := d.Query(`SELECT `+modelCols+` FROM models WHERE id LIKE ? OR display_name LIKE ? OR description LIKE ? OR tags LIKE ? OR aliases LIKE ? OR family LIKE ? ORDER BY status, provider_id`, q, q, q, q, q, q)
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
