//go:build windows || darwin

package store

import (
	"path/filepath"
	"testing"
)

func TestOmllKeepAliveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	oldLegacyConfigPath := legacyConfigPath
	legacyConfigPath = filepath.Join(tmpDir, "config.json")
	defer func() { legacyConfigPath = oldLegacyConfigPath }()

	db, err := newDatabase(filepath.Join(tmpDir, "db.sqlite"))
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// A fresh database gets keep_alive from omllAddKeepAliveColumn, not from
	// upstream's CREATE TABLE literal.
	s, err := db.getSettings()
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if s.KeepAlive != "" {
		t.Errorf("expected empty keep-alive on a new database, got %q", s.KeepAlive)
	}

	s.KeepAlive = "60m"
	if err := db.setSettings(s); err != nil {
		t.Fatalf("failed to set settings: %v", err)
	}

	got, err := db.getSettings()
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if got.KeepAlive != "60m" {
		t.Errorf("expected keep-alive %q, got %q", "60m", got.KeepAlive)
	}
}

// TestOmllRepairSchemaDivergence covers databases written by Oh My Llama
// <= 0.32.14, whose fork migration added keep_alive and stamped schema_version
// 17 before upstream used 17 for onboarding_version. Opening one of those must
// replay upstream's V16->V17 rather than skip it forever.
func TestOmllRepairSchemaDivergence(t *testing.T) {
	tmpDir := t.TempDir()
	oldLegacyConfigPath := legacyConfigPath
	legacyConfigPath = filepath.Join(tmpDir, "config.json")
	defer func() { legacyConfigPath = oldLegacyConfigPath }()

	dbPath := filepath.Join(tmpDir, "db.sqlite")
	db, err := newDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// Rewind a current database into the diverged shape: keep_alive present and
	// populated, everything upstream added at 17 and above gone, version 17.
	for _, stmt := range []string{
		`UPDATE settings SET keep_alive = '60m'`,
		`ALTER TABLE settings DROP COLUMN onboarding_version`,
		`ALTER TABLE settings DROP COLUMN claude_desktop_used`,
		`UPDATE settings SET schema_version = 17`,
	} {
		if _, err := db.conn.Exec(stmt); err != nil {
			t.Fatalf("failed to stage diverged database (%s): %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close database: %v", err)
	}

	// Reopening is what an upgrade does.
	db, err = newDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen diverged database: %v", err)
	}
	defer db.Close()

	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != currentSchemaVersion {
		t.Errorf("expected schema version %d after repair, got %d", currentSchemaVersion, version)
	}

	// getSettings selects onboarding_version and claude_desktop_used, so it fails
	// outright if the replay did not happen.
	s, err := db.getSettings()
	if err != nil {
		t.Fatalf("failed to get settings after repair: %v", err)
	}
	if s.OnboardingVersion != CurrentOnboardingVersion {
		t.Errorf("expected onboarding version %d after repair, got %d", CurrentOnboardingVersion, s.OnboardingVersion)
	}
	// The repair must not cost the user their stored keep-alive.
	if s.KeepAlive != "60m" {
		t.Errorf("expected keep-alive %q to survive the repair, got %q", "60m", s.KeepAlive)
	}
}

// TestOmllRepairSchemaLeavesUpstreamDatabasesAlone guards the fingerprint: a
// genuine upstream database at 17 has onboarding_version and no keep_alive, and
// must not be rewound.
func TestOmllRepairSchemaLeavesUpstreamDatabasesAlone(t *testing.T) {
	tmpDir := t.TempDir()
	oldLegacyConfigPath := legacyConfigPath
	legacyConfigPath = filepath.Join(tmpDir, "config.json")
	defer func() { legacyConfigPath = oldLegacyConfigPath }()

	db, err := newDatabase(filepath.Join(tmpDir, "db.sqlite"))
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	if _, err := db.conn.Exec(`UPDATE settings SET schema_version = 17`); err != nil {
		t.Fatalf("failed to set schema version: %v", err)
	}
	if err := db.omllRepairSchema(); err != nil {
		t.Fatalf("repair failed: %v", err)
	}

	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 17 {
		t.Errorf("expected schema version to stay at 17, got %d", version)
	}
}
