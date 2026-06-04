# Design: Foundation Cleanup

## Technical Approach

Four independent fixes: (1) new SQL migration renames `snapshots.data` → `snapshots.content`, (2) extract `ConfigDir()` to `internal/config` so both `db.DefaultPath()` and `cli.OpenCodeConfigDir()` share one resolution, (3) new `internal/cli/agents.go` for `okit agents list|get|delete`, (4) delete stale docs.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|----------|--------|-------------|-----------|
| Config extraction | New `internal/config` package | Import CLI from DB (circular) | Breaks import cycle, single source of truth |
| Migration approach | New 000002 migration | Alter existing 000001 | golang-migrate expects sequential files; altering committed migration breaks checksums |
| Agents CLI pattern | Inline cobra commands (like snapshots.go) | Dedicated sub-command files (like providers.go) | Agents has only 3 simple operations — no need for separate files per subcommand |

## Data Flow

```
CLI (agents.go) ──→ openDB() ──→ DB agents.go CRUD ──→ SQLite agents table
                                                        ↑
                                                        │
Config path:  internal/config.ConfigDir() ←── db.DefaultPath()
              ←── cli.OpenCodeConfigDir()
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/db/migrations/000002_fix_snapshot_column.up.sql` | Create | `ALTER TABLE snapshots RENAME COLUMN data TO content;` |
| `internal/db/migrations/000002_fix_snapshot_column.down.sql` | Create | `ALTER TABLE snapshots RENAME COLUMN content TO data;` |
| `internal/config/paths.go` | Create | `func ConfigDir() string` — env var → XDG → HOME fallback |
| `internal/db/db.go` | Modify | `DefaultPath()` calls `config.ConfigDir()` instead of `os.UserConfigDir()` |
| `internal/cli/configpath.go` | Modify | `OpenCodeConfigDir()` delegates to `config.ConfigDir()` |
| `internal/cli/agents.go` | Create | `newAgentsCmd()` with list/get/delete subcommands |
| `internal/cli/root.go` | Modify | `root.AddCommand(newAgentsCmd(&dbPath))` |
| `HANDOFF.md` | Delete | Superseded |
| `STALE_TESTS.md` | Delete | Superseded |

## Interfaces

**internal/config/paths.go** — pure function, no side effects, no external deps beyond stdlib:
```go
package config
func ConfigDir() string  // checks OPENCODE_CONFIG_DIR, XDG_CONFIG_HOME, $HOME/.config/opencode
```

**internal/cli/agents.go** — follows snapshots.go pattern:
```go
func newAgentsCmd(dbPath *string) *cobra.Command
// Subcommands: list (all agents, table output), get <id> (full detail), delete <id> (confirm + delete)
```

## Testing

| Layer | What | Approach |
|-------|------|----------|
| Unit | `ConfigDir()` resolution | Table-driven test with env var set/unset |
| Migration | 000002 up + down | Open in-memory DB, apply 000001, apply 000002, verify column name, verify down reverts |
| Manual | `okit agents list` | Smoke test against existing DB with agents |

## Migration / Rollout

No migration required for existing data — `RENAME COLUMN` is safe in modernc.org/sqlite. Schema version in `schema_migrations` table tracks 000001→000002 automatically via golang-migrate.

## Open Questions

None.
