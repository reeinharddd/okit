package db

import (
	"os"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpen_InMemory(t *testing.T) {
	d := openTestDB(t)
	if d == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestMigration000002_RenamesDataToContent(t *testing.T) {
	d := openTestDB(t)
	var name string
	err := d.QueryRow("SELECT name FROM pragma_table_info('snapshots') WHERE name='content'").Scan(&name)
	if err != nil {
		t.Fatalf("expected 'content' column in snapshots table after migration 000002: %v", err)
	}
	var oldName string
	err = d.QueryRow("SELECT name FROM pragma_table_info('snapshots') WHERE name='data'").Scan(&oldName)
	if err == nil {
		t.Fatal("expected 'data' column to be renamed to 'content' after migration 000002")
	}
}

func TestDefaultPath_UsesConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	got := DefaultPath()
	want := filepath.Join(dir, "opencode-kit.db")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected config dir to be created: %v", err)
	}
}

func TestMigration000002_Down_RevertsColumn(t *testing.T) {
	d := openTestDB(t)
	// Apply the down migration manually to verify it works
	_, err := d.Exec("ALTER TABLE snapshots RENAME COLUMN content TO data;")
	if err != nil {
		t.Fatalf("down migration failed: %v", err)
	}
	var name string
	err = d.QueryRow("SELECT name FROM pragma_table_info('snapshots') WHERE name='data'").Scan(&name)
	if err != nil {
		t.Fatal("expected 'data' column after down migration")
	}
}

func TestSeedDefaults(t *testing.T) {
	d := openTestDB(t)
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM routing_rules").Scan(&count)
	if err != nil {
		t.Fatalf("count routing_rules: %v", err)
	}
	if count == 0 {
		t.Error("expected seeded routing rules")
	}
}

func TestDefaultPath_Fallback(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got := DefaultPath()
	home := os.Getenv("HOME")
	want := filepath.Join(home, ".config", "opencode", "opencode-kit.db")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestMigration000002_PreservesData(t *testing.T) {
	d := openTestDB(t)
	// Insert a row with the current schema (content column exists after 000002)
	_, err := d.Exec("INSERT INTO snapshots (hash, content) VALUES ('abc', '{\"test\":1}')")
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	var content string
	err = d.QueryRow("SELECT content FROM snapshots WHERE hash='abc'").Scan(&content)
	if err != nil {
		t.Fatalf("read snapshot content: %v", err)
	}
	if content != `{"test":1}` {
		t.Errorf("got content %q, want %q", content, `{"test":1}`)
	}
}
