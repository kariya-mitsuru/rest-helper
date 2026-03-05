// SPDX-License-Identifier: MIT

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func Init() error {
	dataDir, err := dataPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "rest-helper.db")
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("setting WAL mode: %w", err)
	}

	return migrate()
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func dataPath() (string, error) {
	if dir := os.Getenv("REST_HELPER_DATA_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "rest-helper"), nil
}

func migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			method TEXT NOT NULL,
			url TEXT NOT NULL,
			request_headers TEXT DEFAULT '{}',
			request_body TEXT DEFAULT '',
			status_code INTEGER DEFAULT 0,
			response_headers TEXT DEFAULT '{}',
			response_body TEXT DEFAULT '',
			response_time_ms INTEGER DEFAULT 0,
			response_size INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_history_created_at ON history(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_history_url ON history(url)`,
		// Migration: add body_format column for existing DBs
		`ALTER TABLE history ADD COLUMN body_format TEXT DEFAULT 'JSON'`,
		// Migration: add proto and status columns
		`ALTER TABLE history ADD COLUMN response_proto TEXT DEFAULT ''`,
		`ALTER TABLE history ADD COLUMN response_status TEXT DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// Ignore "duplicate column" errors from ALTER TABLE migrations
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
	}

	return nil
}
