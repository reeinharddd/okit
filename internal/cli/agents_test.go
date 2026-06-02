package cli

import (
	"path/filepath"
	"testing"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

func seedTestAgents(t *testing.T, dbPath string) {
	t.Helper()
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer d.Close()
	agents := []*models.Agent{
		{ID: "coder", TaskType: "coding", CurrentModelID: "gpt-4", Status: "active", Mode: "subagent", Description: "Coding agent"},
		{ID: "reviewer", TaskType: "review", CurrentModelID: "claude-3", Status: "active", Mode: "subagent", Description: "Review agent"},
		{ID: "temp-agent", TaskType: "temp", Status: "active", Mode: "subagent", Description: "Temporary agent"},
	}
	for _, a := range agents {
		if err := d.UpsertAgent(a); err != nil {
			t.Fatalf("upsert agent %s: %v", a.ID, err)
		}
	}
}

func TestAgentsList_PrintsAllAgents(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	seedTestAgents(t, dbPath)

	cmd := newAgentsCmd(&dbPath)
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAgentsGet_PrintsAgentDetails(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	seedTestAgents(t, dbPath)

	cmd := newAgentsCmd(&dbPath)
	getCmd, _, err := cmd.Find([]string{"get"})
	if err != nil {
		t.Fatal(err)
	}
	if err := getCmd.RunE(getCmd, []string{"coder"}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentsGet_NonExistentReturnsError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	seedTestAgents(t, dbPath)

	cmd := newAgentsCmd(&dbPath)
	getCmd, _, err := cmd.Find([]string{"get"})
	if err != nil {
		t.Fatal(err)
	}
	if err := getCmd.RunE(getCmd, []string{"nonexistent"}); err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestAgentsDelete_RemovesAgent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	seedTestAgents(t, dbPath)

	cmd := newAgentsCmd(&dbPath)
	delCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatal(err)
	}
	if err := delCmd.RunE(delCmd, []string{"temp-agent"}); err != nil {
		t.Fatal(err)
	}

	// Verify deleted
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.GetAgent("temp-agent"); err == nil {
		t.Fatal("expected agent to be deleted")
	}
}

func TestAgentsDelete_NonExistentReturnsError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	cmd := newAgentsCmd(&dbPath)
	delCmd, _, err := cmd.Find([]string{"delete"})
	if err != nil {
		t.Fatal(err)
	}
	if err := delCmd.RunE(delCmd, []string{"missing"}); err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestAgentsList_Empty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "okit.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	cmd := newAgentsCmd(&dbPath)
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatal(err)
	}
}
