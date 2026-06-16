# opencode-kit — Development Guide

## Project
Go CLI tool managing OpenCode configuration (cobra + SQLite + concurrent API calls).
Vision: **auto-orchestrator** — manages models, skills, MCPs, agents, routing, and runtime config dynamically based on user's model arsenal and harness sources.

## Commands
- `make test` — `go test -v ./...`
- `make lint` — `go vet ./...`
- `make build` — `go build -o okit ./cmd/okit/`
- `go test -race -coverprofile=coverage.out ./...`
- `go tool cover -func=coverage.out`
- `bash test-suite.sh` — integration tests

## Workflow: SDD-TDD Hybrid

**Phase 0 (Build Fix — one-time):** Systematic fix. interface → crud → 9 broken packages → stale tests.
**Phases 1-6:** SDD for planning, TDD for coding.

| Step | Action | Skills | Agent |
|------|--------|--------|-------|
| 1 | SDD Explore — load context, search memory, understand codebase | — | direct |
| 2 | SDD Propose — define scope, intent, approach, affected areas | `golang-pro` | direct |
| 3 | SDD Spec — requirements, capabilities, acceptance scenarios | — | direct |
| 4 | SDD Design — interfaces, types, architecture decisions | `architecture-designer`, `golang-pro` | direct |
| 5 | SDD Tasks — break into implementation tasks | — | direct |
| 6 | SDD Apply + TDD — ONE test → ONE impl → repeat per task | `golang-pro`, `go-testing`, `tdd` | `coder` (>3 files) |
| 7 | SDD Verify — `go vet ./... && go test -race ./... && go build ./...` | — | `devops` |
| 8 | SDD Archive — save phase summary to Engram | — | direct |

**TDD Rules (within Apply):**
- NEVER write all tests first (horizontal slices anti-pattern)
- ONE test → ONE impl → repeat (vertical slices via tracer bullets)
- Test behavior through public interfaces, not implementation details
- Only enough code to pass current test — no speculative features

## Skills per Stage
| Stage | Skills |
|-------|--------|
| Architecture/Design | `architecture-designer`, `golang-pro` |
| Implementation | `golang-pro`, `go-testing`, `tdd` |
| Review | `code-reviewer` |
| Debug | `diagnose` |

## Agents
| Task | Agent | When |
|------|-------|------|
| Implementation | `coder` | >3 files changed |
| Simple edits | direct | 1-2 files |
| Code review | `reviewer` | Pre-merge |
| Debugging | `debugger` | Test failures |
| CI/config | `devops` | Pipeline work |

## Test Conventions
- Table-driven tests with descriptive names (`TestFuncName_Scenario_Expected`)
- External test packages (`package db_test`)
- SQLite in-memory for DB tests: `db.Open(":memory:")`
- Interface-based mocking (no framework)

## Stale Tests
All stale test files have been removed. The test suite passes cleanly.

## Memory
Save to engram with `topic_key: architecture/okit-workflow` or `architecture/okit-<feature>`.
Phase completions always saved as `sessions/okit-phase-<N>-complete`.

## Full Reference
See [DEVELOPMENT.md](./DEVELOPMENT.md) for the complete workflow specification.
See [HANDOFF.md](./HANDOFF.md) for the 6-phase implementation plan.
