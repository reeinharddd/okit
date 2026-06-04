package db

import (
	"github.com/reeinharddd/okit/pkg/models"
)

func (d *DB) UpsertRoutingRule(r *models.RoutingRule) error {
	_, err := d.Exec(`INSERT INTO routing_rules (task_key, description, min_context, needs_fc, needs_vision, max_cost_per_call, current_model_id, fallback_ids, last_assigned)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_key) DO UPDATE SET
		description=excluded.description, min_context=excluded.min_context,
		needs_fc=excluded.needs_fc, needs_vision=excluded.needs_vision,
		max_cost_per_call=excluded.max_cost_per_call,
		current_model_id=excluded.current_model_id, fallback_ids=excluded.fallback_ids, last_assigned=excluded.last_assigned`,
		r.TaskKey, r.Description, r.MinContext, boolToInt(r.NeedsFC), boolToInt(r.NeedsVision),
		r.MaxCostPerCall, r.CurrentModelID, r.FallbackIDs, r.LastAssigned)
	return err
}

func (d *DB) ListRoutingRules() ([]models.RoutingRule, error) {
	rows, err := d.Query(`SELECT task_key, COALESCE(description,''), COALESCE(min_context,0), COALESCE(needs_fc,0), COALESCE(needs_vision,0), COALESCE(max_cost_per_call,0), COALESCE(current_model_id,''), COALESCE(fallback_ids,''), COALESCE(last_assigned,0) FROM routing_rules ORDER BY task_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.RoutingRule
	for rows.Next() {
		var r models.RoutingRule
		var fc, vis int
		if err := rows.Scan(&r.TaskKey, &r.Description, &r.MinContext, &fc, &vis, &r.MaxCostPerCall, &r.CurrentModelID, &r.FallbackIDs, &r.LastAssigned); err != nil {
			return nil, err
		}
		r.NeedsFC = fc != 0
		r.NeedsVision = vis != 0
		out = append(out, r)
	}
	return out, nil
}

func (d *DB) GetRoutingRule(key string) (*models.RoutingRule, error) {
	var r models.RoutingRule
	var fc, vis int
	err := d.QueryRow(`SELECT task_key, COALESCE(description,''), COALESCE(min_context,0), COALESCE(needs_fc,0), COALESCE(needs_vision,0), COALESCE(max_cost_per_call,0), COALESCE(current_model_id,''), COALESCE(fallback_ids,''), COALESCE(last_assigned,0) FROM routing_rules WHERE task_key=?`, key).
		Scan(&r.TaskKey, &r.Description, &r.MinContext, &fc, &vis, &r.MaxCostPerCall, &r.CurrentModelID, &r.FallbackIDs, &r.LastAssigned)
	if err != nil {
		return nil, err
	}
	r.NeedsFC = fc != 0
	r.NeedsVision = vis != 0
	return &r, nil
}

func (d *DB) InsertRoutingEvent(event *models.RoutingEvent) error {
	_, err := d.Exec(`INSERT INTO routing_events (task_key, selected_model, candidates, reason, shadow) VALUES (?, ?, ?, ?, ?)`,
		event.TaskKey, event.SelectedModel, event.Candidates, event.Reason, boolToInt(event.Shadow))
	return err
}

func (d *DB) ListRoutingEvents(limit int) ([]models.RoutingEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.Query(`SELECT id, COALESCE(task_key,''), COALESCE(selected_model,''), COALESCE(candidates,''), COALESCE(reason,''), COALESCE(shadow,0), COALESCE(created_at,'') FROM routing_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.RoutingEvent
	for rows.Next() {
		var e models.RoutingEvent
		var shadow int
		if err := rows.Scan(&e.ID, &e.TaskKey, &e.SelectedModel, &e.Candidates, &e.Reason, &shadow, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Shadow = shadow != 0
		out = append(out, e)
	}
	return out, nil
}

func (d *DB) UpsertBudget(b *models.BudgetConfig) error {
	_, err := d.Exec(`INSERT INTO budget_config (id, daily_global_usd, preferred_tier, updated_at) VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET daily_global_usd=excluded.daily_global_usd, preferred_tier=excluded.preferred_tier, updated_at=datetime('now')`,
		b.ID, b.DailyGlobalUSD, b.PreferredTier)
	return err
}

func (d *DB) GetBudget() (*models.BudgetConfig, error) {
	var b models.BudgetConfig
	err := d.QueryRow(`SELECT id, daily_global_usd, preferred_tier, COALESCE(updated_at,'') FROM budget_config WHERE id='default'`).
		Scan(&b.ID, &b.DailyGlobalUSD, &b.PreferredTier, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (d *DB) InsertSyncLog(phase, status, details string, durationMs int64) error {
	_, err := d.Exec(`INSERT INTO sync_log (phase, status, details, duration_ms) VALUES (?, ?, ?, ?)`,
		phase, status, details, durationMs)
	return err
}

func (d *DB) ListSyncLogs(limit int) ([]models.SyncLog, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.Query(`SELECT id, COALESCE(phase,''), COALESCE(status,''), COALESCE(details,''), COALESCE(duration_ms,0), COALESCE(created_at,'') FROM sync_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SyncLog
	for rows.Next() {
		var l models.SyncLog
		if err := rows.Scan(&l.ID, &l.Phase, &l.Status, &l.Details, &l.DurationMs, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func (d *DB) InsertExecLog(l *models.ExecLog) error {
	_, err := d.Exec(`INSERT INTO exec_log (agent, model, task, tokens_in, tokens_out, duration_ms, success, error) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Agent, l.Model, l.Task, l.TokensIn, l.TokensOut, l.DurationMs, boolToInt(l.Success), l.Error)
	return err
}

func (d *DB) ListExecLogs(limit int) ([]models.ExecLog, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.Query(`SELECT id, COALESCE(agent,''), COALESCE(model,''), COALESCE(task,''), COALESCE(tokens_in,0), COALESCE(tokens_out,0), COALESCE(duration_ms,0), COALESCE(success,1), COALESCE(error,''), COALESCE(created_at,'') FROM exec_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ExecLog
	for rows.Next() {
		var l models.ExecLog
		var succ int
		if err := rows.Scan(&l.ID, &l.Agent, &l.Model, &l.Task, &l.TokensIn, &l.TokensOut, &l.DurationMs, &succ, &l.Error, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Success = succ != 0
		out = append(out, l)
	}
	return out, nil
}

func (d *DB) InsertSnapshot(hash, content string) error {
	_, err := d.Exec(`INSERT INTO snapshots (hash, content) VALUES (?, ?)`, hash, content)
	return err
}

func (d *DB) ListSnapshots(limit int) ([]models.Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.Query(`SELECT id, hash, content, COALESCE(created_at,'') FROM snapshots ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Snapshot
	for rows.Next() {
		var s models.Snapshot
		if err := rows.Scan(&s.ID, &s.Hash, &s.Content, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (d *DB) GetSnapshot(id int64) (*models.Snapshot, error) {
	var s models.Snapshot
	err := d.QueryRow(`SELECT id, hash, content, COALESCE(created_at,'') FROM snapshots WHERE id=?`, id).
		Scan(&s.ID, &s.Hash, &s.Content, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) DeleteSnapshot(id int64) error {
	_, err := d.Exec(`DELETE FROM snapshots WHERE id=?`, id)
	return err
}

func (d *DB) SetPreference(key, value string) error {
	_, err := d.Exec(`INSERT INTO preferences (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (d *DB) CleanupProviderPrefs() (int, error) {
	res, err := d.Exec(`DELETE FROM preferences WHERE key LIKE 'config/provider_%'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (d *DB) CleanupInvalidPreferences() (int, error) {
	res, err := d.Exec(`DELETE FROM preferences WHERE value='null' OR value='NULL' OR value='<nil>'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (d *DB) GetPreference(key string) (string, error) {
	var val string
	err := d.QueryRow(`SELECT value FROM preferences WHERE key=?`, key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (d *DB) DeletePreference(key string) error {
	_, err := d.Exec(`DELETE FROM preferences WHERE key=?`, key)
	return err
}

func (d *DB) ListPreferences() (map[string]string, error) {
	rows, err := d.Query(`SELECT key, value FROM preferences ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}
