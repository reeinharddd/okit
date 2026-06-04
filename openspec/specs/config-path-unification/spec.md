# Config Path Unification Specification

## Purpose

Unify the database path resolution so `db.DefaultPath()` respects `$OPENCODE_CONFIG_DIR`, matching the behavior of `cli.OpenCodeConfigDir()`. Extract shared logic into `internal/config` to avoid circular imports.

## Requirements

### Requirement: Shared Config Directory

The system SHALL provide `internal/config.ConfigDir()` that checks `$OPENCODE_CONFIG_DIR`, then `$XDG_CONFIG_HOME/opencode`, then falls back to `$HOME/.config/opencode`.

#### Scenario: Env var respected

- GIVEN `OPENCODE_CONFIG_DIR` is set to `/custom/path`
- WHEN `ConfigDir()` is called
- THEN it returns `/custom/path`

#### Scenario: Fallback resolution

- GIVEN no env vars are set
- WHEN `ConfigDir()` is called
- THEN it returns `$HOME/.config/opencode`

### Requirement: DefaultPath delegation

The system SHALL modify `db.DefaultPath()` to call `internal/config.ConfigDir()` instead of `os.UserConfigDir()`.

#### Scenario: DB path matches config dir

- GIVEN `OPENCODE_CONFIG_DIR` is set
- WHEN `db.DefaultPath()` is called
- THEN the returned path is inside that directory
