package db

import (
	"fmt"

	"github.com/reeinharddd/okit/pkg/models"
)

func (d *DB) UpsertAgent(a *models.Agent) error {
	_, err := d.Exec(`INSERT INTO agents (id, task_type, description, current_model_id, fallback_ids, prompt_file, temperature, max_steps, permission, color, mode, hidden, status, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		task_type=excluded.task_type, description=excluded.description,
		current_model_id=excluded.current_model_id, fallback_ids=excluded.fallback_ids,
		prompt_file=excluded.prompt_file, temperature=excluded.temperature,
		max_steps=excluded.max_steps, permission=excluded.permission,
		color=excluded.color, mode=excluded.mode, hidden=excluded.hidden,
		status=excluded.status, source=excluded.source`,
		a.ID, a.TaskType, a.Description, a.CurrentModelID, a.FallbackIDs,
		a.PromptFile, a.Temperature, a.MaxSteps, a.Permission, a.Color,
		a.Mode, boolToInt(a.Hidden), a.Status, a.Source)
	return err
}

func (d *DB) ListAgents() ([]models.Agent, error) {
	rows, err := d.Query(`SELECT id, COALESCE(task_type,''), COALESCE(description,''), COALESCE(current_model_id,''), COALESCE(fallback_ids,''), COALESCE(prompt_file,''), COALESCE(temperature,0.3), COALESCE(max_steps,0), COALESCE(permission,''), COALESCE(color,''), COALESCE(mode,'subagent'), COALESCE(hidden,0), COALESCE(status,'active'), COALESCE(source,'auto') FROM agents ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Agent
	for rows.Next() {
		var a models.Agent
		var hid int
		if err := rows.Scan(&a.ID, &a.TaskType, &a.Description, &a.CurrentModelID, &a.FallbackIDs, &a.PromptFile, &a.Temperature, &a.MaxSteps, &a.Permission, &a.Color, &a.Mode, &hid, &a.Status, &a.Source); err != nil {
			return nil, err
		}
		a.Hidden = hid != 0
		out = append(out, a)
	}
	return out, nil
}

func (d *DB) GetAgent(id string) (*models.Agent, error) {
	var a models.Agent
	var hid int
	err := d.QueryRow(`SELECT id, COALESCE(task_type,''), COALESCE(description,''), COALESCE(current_model_id,''), COALESCE(fallback_ids,''), COALESCE(prompt_file,''), COALESCE(temperature,0.3), COALESCE(max_steps,0), COALESCE(permission,''), COALESCE(color,''), COALESCE(mode,'subagent'), COALESCE(hidden,0), COALESCE(status,'active'), COALESCE(source,'auto') FROM agents WHERE id=?`, id).
		Scan(&a.ID, &a.TaskType, &a.Description, &a.CurrentModelID, &a.FallbackIDs, &a.PromptFile, &a.Temperature, &a.MaxSteps, &a.Permission, &a.Color, &a.Mode, &hid, &a.Status, &a.Source)
	if err != nil {
		return nil, err
	}
	a.Hidden = hid != 0
	return &a, nil
}

func (d *DB) DeleteAgent(id string) error {
	res, err := d.Exec(`DELETE FROM agents WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agent %q not found", id)
	}
	return nil
}
