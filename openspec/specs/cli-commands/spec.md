# CLI Commands Specification

## Purpose

The `okit` CLI manages OpenCode infrastructure. Existing commands include `discover`, `audit`, `generate`, `status`, `snapshots`, `preferences`, and others. This change adds the `agents` subcommand with `list`, `get`, and `delete` operations backed by existing DB CRUD in `internal/db/agents.go`.

## Requirements

### Requirement: Agents subcommand

The system SHALL provide `okit agents` with subcommands `list`, `get <id>`, and `delete <id>`.

#### Scenario: List agents

- GIVEN the database has agents
- WHEN the user runs `okit agents list`
- THEN all agents are printed with ID, model, status, and task type

#### Scenario: Get agent by ID

- GIVEN an agent with id `coder` exists
- WHEN the user runs `okit agents get coder`
- THEN the agent's full details are printed

#### Scenario: Delete agent

- GIVEN an agent with id `temp-agent` exists
- WHEN the user runs `okit agents delete temp-agent`
- THEN the agent is removed from the database
- AND a confirmation message is printed

#### Scenario: Delete non-existent agent

- GIVEN no agent with id `missing` exists
- WHEN the user runs `okit agents delete missing`
- THEN an error is returned
- AND the command exits with non-zero status
