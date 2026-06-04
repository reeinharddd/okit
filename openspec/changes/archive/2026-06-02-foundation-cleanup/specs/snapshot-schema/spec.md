# Snapshot Schema Specification

## Purpose

Fix the `snapshots` schema so the column storing serialized snapshot data is named `content` — matching the Go model struct and all query code — instead of the current `data` column.

## Requirements

### Requirement: Rename Column

The system SHALL run migration `000002` that renames `snapshots.data` to `snapshots.content` using `ALTER TABLE ... RENAME COLUMN`.

#### Scenario: Migration applies cleanly

- GIVEN a fresh database with migration `000001` applied
- WHEN migration `000002` runs
- THEN `snapshots` has column `content` instead of `data`

#### Scenario: Existing data survives rename

- GIVEN an existing database with rows in `snapshots` using `data` column
- WHEN migration `000002` runs
- THEN existing row values are preserved under `content` column name

### Requirement: Down Migration

The system SHALL provide a `000002.down.sql` that reverts the rename.

#### Scenario: Rollback works

- GIVEN a database at migration `000002`
- WHEN the down migration runs
- THEN column is renamed back to `data`
