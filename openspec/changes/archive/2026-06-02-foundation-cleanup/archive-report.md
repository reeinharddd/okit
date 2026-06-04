# Archive Report: Foundation Cleanup

**Archived**: 2026-06-02
**Source**: `openspec/changes/foundation-cleanup/`
**Destination**: `openspec/changes/archive/2026-06-02-foundation-cleanup/`

## Summary

Four foundational defects fixed: snapshot column mismatch, config path ignored env var, agents had DB CRUD but no CLI, stale docs lingering.

## What Was Done

| Domain | Action | Details |
|--------|--------|---------|
| `snapshot-schema` | Created (main spec) | Migration `000002` renames `snapshots.data` → `snapshots.content` to match Go struct |
| `config-path-unification` | Created (main spec) | Shared `internal/config.ConfigDir()` breaks circular import, respects `$OPENCODE_CONFIG_DIR` |
| `cli-commands` | Created (main spec) | `okit agents` subcommand with list/get/delete backed by existing DB CRUD |
| Stale docs | Deleted | `HANDOFF.md` and `STALE_TESTS.md` removed |

## Specs Synced to Main

All three delta specs were copied to `openspec/specs/{domain}/spec.md` as new main specs (no prior specs existed).

## Archive Contents

- `proposal.md` ✅ — 4-scope intent, risks, rollback
- `specs/snapshot-schema/spec.md` ✅ — 2 requirements, 3 scenarios
- `specs/config-path-unification/spec.md` ✅ — 2 requirements, 3 scenarios
- `specs/cli-commands/spec.md` ✅ — 1 requirement, 4 scenarios
- `design.md` ✅ — 3 ADRs, files, interfaces
- `tasks.md` ✅ — 21/21 tasks complete, all phases done
- `archive-report.md` ✅ — this file

## Verification Result

All 21 tasks across 5 phases marked complete. Tasks 5.1-5.3 confirm `go vet`, `go test -race`, and `go build` pass.

## Source of Truth Updated

The following main specs now reflect the new behavior:
- `openspec/specs/snapshot-schema/spec.md`
- `openspec/specs/config-path-unification/spec.md`
- `openspec/specs/cli-commands/spec.md`
