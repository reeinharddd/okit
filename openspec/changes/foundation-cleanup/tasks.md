# Tasks: Foundation Cleanup

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~250-300 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

## Phase 1: Snapshot Migration Fix

- [x] 1.1 RED: Test migration 000002 — apply 000001 then 000002, assert `snapshots.content` exists
- [x] 1.2 GREEN: Create `migrations/000002_fix_snapshot_column.up.sql` — `ALTER TABLE snapshots RENAME COLUMN data TO content;`
- [x] 1.3 GREEN: Create `migrations/000002_fix_snapshot_column.down.sql` — reverse rename
- [x] 1.4 REFACTOR: Verify down migration — apply 000002 then down, assert column `data` restored

## Phase 2: Config Path Unification

- [x] 2.1 RED: Test `config.ConfigDir()` — env var set, XDG fallback, HOME fallback
- [x] 2.2 GREEN: Create `internal/config/paths.go` — `func ConfigDir() string` checks `$OPENCODE_CONFIG_DIR` → `$XDG_CONFIG_HOME/opencode` → `$HOME/.config/opencode`
- [x] 2.3 RED: Test `db.DefaultPath()` uses `config.ConfigDir()` via env var
- [x] 2.4 GREEN: Modify `internal/db/db.go` — `DefaultPath()` calls `config.ConfigDir()`
- [x] 2.5 GREEN: Modify `internal/cli/configpath.go` — `OpenCodeConfigDir()` delegates to `config.ConfigDir()`

## Phase 3: Agents CLI Command

- [x] 3.1 RED: Test `okit agents list` prints agents from DB
- [x] 3.2 RED: Test `okit agents get <id>` prints agent details
- [x] 3.3 RED: Test `okit agents delete <id>` removes agent + confirms
- [x] 3.4 RED: Test `okit agents delete <missing>` returns error + non-zero exit
- [x] 3.5 GREEN: Create `internal/cli/agents.go` — `newAgentsCmd()` with list/get/delete
- [x] 3.6 GREEN: Register `newAgentsCmd(&dbPath)` in `internal/cli/root.go`

## Phase 4: Stale Docs Cleanup

- [x] 4.1 Delete `HANDOFF.md`
- [x] 4.2 Delete `STALE_TESTS.md`

## Phase 5: Verify

- [x] 5.1 `go vet ./...` — clean
- [x] 5.2 `go test -race ./...` — all pass
- [x] 5.3 `go build ./...` — compiles
