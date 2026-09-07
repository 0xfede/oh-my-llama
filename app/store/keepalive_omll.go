package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// The fork's keep_alive setting lives here rather than inline in database.go.
//
// Upstream owns currentSchemaVersion, the settings CREATE TABLE literal, the
// migrateVnToVn+1 chain and the long column lists in getSettings/setSettings.
// Every upstream release touches at least one of those, so a patch that edits
// them conflicts on every rebase - which is exactly what stalled the sync job
// on v0.33.3.
//
// Worse, a fork migration wedged into upstream's numbered chain diverges
// silently. Oh My Llama <= 0.32.14 shipped its own migrateV16ToV17 that added
// keep_alive and stamped schema_version 17. Upstream later shipped a *different*
// V16->V17 that adds onboarding_version, so those databases sit at 17 with
// keep_alive but without onboarding_version, and the migration loop - which only
// runs cases at or above the stored version - would skip upstream's V16->V17
// forever. The first getSettings after such an upgrade fails with "no such
// column: onboarding_version". omllRepairSchema below rewinds those rows so
// upstream's own migration replays.

// omllAddKeepAliveColumn adds the fork's keep_alive column. It is idempotent, so
// it serves both fresh databases (keep_alive is deliberately absent from
// upstream's CREATE TABLE literal) and every existing one. Runs after upstream's
// migrate().
func (db *database) omllAddKeepAliveColumn() error {
	_, err := db.conn.Exec(`ALTER TABLE settings ADD COLUMN keep_alive TEXT NOT NULL DEFAULT ''`)
	if err != nil && !duplicateColumnError(err) {
		return fmt.Errorf("add keep_alive column: %w", err)
	}
	return nil
}

// omllRepairSchema rewinds databases written by Oh My Llama <= 0.32.14, whose
// fork migration claimed schema version 17 before upstream used it for
// onboarding_version. Runs before upstream's migrate() so the rewound version is
// what the migration loop reads.
//
// The fingerprint is unambiguous: only an omll database can be at exactly 17
// with keep_alive present and onboarding_version missing. Rewinding to 16 makes
// upstream replay V16->V17 and then V17->V18; both only add columns and both
// tolerate a column that already exists, so keep_alive and its stored value
// survive untouched.
func (db *database) omllRepairSchema() error {
	version, err := db.getSchemaVersion()
	if err != nil {
		// A brand new database has no settings row yet. Nothing to repair.
		return nil
	}
	if version != 17 {
		return nil
	}

	hasKeepAlive, err := db.omllHasSettingsColumn("keep_alive")
	if err != nil {
		return err
	}
	hasOnboarding, err := db.omllHasSettingsColumn("onboarding_version")
	if err != nil {
		return err
	}
	if !hasKeepAlive || hasOnboarding {
		return nil
	}

	slog.Info("repairing Oh My Llama schema divergence: rewinding to 16 so upstream's v16->v17 migration replays")
	if _, err := db.conn.Exec(`UPDATE settings SET schema_version = 16`); err != nil {
		return fmt.Errorf("rewind schema version: %w", err)
	}
	return nil
}

// omllHasSettingsColumn reports whether the settings table has the named column.
// A bare SELECT is used rather than PRAGMA table_info so this does not depend on
// which introspection features the bundled SQLite is built with. column is
// always a literal from this file, never user input.
func (db *database) omllHasSettingsColumn(column string) (bool, error) {
	var discard any
	err := db.conn.QueryRow(`SELECT ` + column + ` FROM settings LIMIT 1`).Scan(&discard)
	switch {
	case err == nil, errors.Is(err, sql.ErrNoRows):
		// No rows still means the column resolved.
		return true, nil
	case columnNotExists(err):
		return false, nil
	default:
		return false, fmt.Errorf("probe settings column %s: %w", column, err)
	}
}

// omllGetKeepAlive and omllSetKeepAlive are read and written separately from
// upstream's combined settings row so this patch never appears in the SELECT and
// UPDATE column lists, which churn on nearly every upstream release.
func (db *database) omllGetKeepAlive() (string, error) {
	var keepAlive string
	if err := db.conn.QueryRow(`SELECT keep_alive FROM settings`).Scan(&keepAlive); err != nil {
		return "", fmt.Errorf("get keep_alive: %w", err)
	}
	return keepAlive, nil
}

func (db *database) omllSetKeepAlive(keepAlive string) error {
	if _, err := db.conn.Exec(`UPDATE settings SET keep_alive = ?`, keepAlive); err != nil {
		return fmt.Errorf("set keep_alive: %w", err)
	}
	return nil
}
