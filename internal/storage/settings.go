// SPDX-License-Identifier: MIT

package storage

import "database/sql"

// Setting key constants.
const (
	KeyBodyFormat     = "body_format"
	KeyResponseFormat = "response_format"
	KeyResponseWrap   = "response_wrap"
)

// GetSetting returns the value for the given key, or "" if not found.
func GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting stores a key-value pair (INSERT OR REPLACE).
func SetSetting(key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}
