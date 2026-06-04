# Proposal: Foundation Cleanup

## Intent

Fix four concrete defects: snapshot column `data`/`content` mismatch causes SQL error on insert; DB path ignores `$OPENCODE_CONFIG_DIR`; agents have DB CRUD but no CLI; stale docs misrepresent current state.

## Scope

### In Scope
- Migration: rename `snapshots.data` → `snapshots.content`
- Unify `db.DefaultPath()` with `cli.OpenCodeConfigDir()` via shared `internal/config`
- Add `agents` CLI (list/get/delete)
- Delete `HANDOFF.md` and `STALE_TESTS.md`

### Out of Scope
- Rewriting stale tests (documented in STALE_TESTS.md)
- Other missing CLI (snapshots/prefs already exist)

## Capabilities

### New Capabilities
- `snapshot-schema`: Fix column name `data` → `content` to match Go code + model struct
- `config-path-unification`: Make `DefaultPath()` respect `$OPENCODE_CONFIG_DIR`

### Modified Capabilities
- `cli-commands`: Add `okit agents` subcommand (DB ops exist)

## Approach

1. **Snapshot**: New migration `000002_fix_snapshot_column.up.sql` renames `data` → `content`.
2. **Config path**: Extract `ConfigDir()` to `internal/config/paths.go`. Both `db` + `cli` import it. `DefaultPath()` delegates to it.
3. **Agents CLI**: New `internal/cli/agents.go` with `newAgentsCmd()`. Register in `root.go`. Subcommands: `list`, `get <id>`, `delete <id>`.
4. **Stale docs**: `rm HANDOFF.md STALE_TESTS.md`.

## Affected Areas

| Area | Impact | What |
|------|--------|------|
| `internal/db/migrations/000002_*.sql` | New | Rename column |
| `internal/db/db.go` | Modified | `DefaultPath()` uses shared config |
| `internal/config/paths.go` | New | Shared `ConfigDir()` |
| `internal/cli/configpath.go` | Modified | Delegate to internal/config |
| `internal/cli/agents.go` | New | Agents CLI |
| `internal/cli/root.go` | Modified | Register agents |
| `HANDOFF.md`, `STALE_TESTS.md` | Removed | Superseded |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Migration fails on existing data | Low | `RENAME COLUMN` safe in modernc.org/sqlite |
| Config refactor breaks path resolution | Low | Extract logic as-is, no behavior change |
| Circular import (db ↔ cli) | Low | `internal/config` breaks the cycle |

## Rollback

- **Schema**: Down migration reverts rename
- **Config**: Revert `internal/config/paths.go` + restore `DefaultPath()`
- **Agents**: Remove `newAgentsCmd()` + `agents.go`
- **Docs**: `git checkout HANDOFF.md STALE_TESTS.md`

## Dependencies

None.

## Success Criteria

- [ ] `InsertSnapshot` runs without SQL error on fresh and existing DBs
- [ ] `OPENCODE_CONFIG_DIR=/tmp/test-config okit status` opens DB there
- [ ] `go vet ./... && go test -race ./...` passes
- [ ] `okit agents list` returns agents from DB
- [ ] `HANDOFF.md` and `STALE_TESTS.md` deleted
