// SPDX-License-Identifier: MIT

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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

	return createTables()
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

func createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			method TEXT NOT NULL,
			url TEXT NOT NULL,
			request_headers TEXT DEFAULT '{}',
			request_body TEXT DEFAULT '',
			body_format TEXT DEFAULT 'JSON',
			status_code INTEGER DEFAULT 0,
			response_proto TEXT DEFAULT '',
			response_status TEXT DEFAULT '',
			response_headers TEXT DEFAULT '{}',
			response_body TEXT DEFAULT '',
			response_time_ms INTEGER DEFAULT 0,
			response_size INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_history_created_at ON history(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_history_url ON history(url)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("create table failed: %w", err)
		}
	}

	return nil
}
