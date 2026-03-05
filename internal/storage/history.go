// SPDX-License-Identifier: MIT

package storage

import (
	"encoding/json"
	"fmt"
	"strings"
)

func SaveHistory(entry *HistoryEntry) error {
	if entry.BodyFormat == "" {
		entry.BodyFormat = "JSON"
	}
	result, err := db.Exec(
		`INSERT INTO history (method, url, request_headers, request_body, body_format, status_code, response_proto, response_status, response_headers, response_body, response_time_ms, response_size)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Method,
		entry.URL,
		entry.RequestHeaders,
		entry.RequestBody,
		entry.BodyFormat,
		entry.StatusCode,
		entry.ResponseProto,
		entry.ResponseStatus,
		entry.ResponseHeaders,
		entry.ResponseBody,
		entry.ResponseTimeMs,
		entry.ResponseSize,
	)
	if err != nil {
		return fmt.Errorf("saving history: %w", err)
	}
	entry.ID, _ = result.LastInsertId()
	return nil
}

const selectColumns = `id, method, url, request_headers, request_body, COALESCE(body_format, 'JSON'), status_code, COALESCE(response_proto, ''), COALESCE(response_status, ''), response_headers, response_body, response_time_ms, response_size, created_at`

func scanEntry(scan func(dest ...any) error) (HistoryEntry, error) {
	var e HistoryEntry
	err := scan(&e.ID, &e.Method, &e.URL, &e.RequestHeaders, &e.RequestBody,
		&e.BodyFormat, &e.StatusCode, &e.ResponseProto, &e.ResponseStatus,
		&e.ResponseHeaders, &e.ResponseBody,
		&e.ResponseTimeMs, &e.ResponseSize, &e.CreatedAt)
	return e, err
}

func ListHistory(limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(
		`SELECT `+selectColumns+` FROM history ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		e, err := scanEntry(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scanning history row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func DeleteHistory(id int64) error {
	_, err := db.Exec("DELETE FROM history WHERE id = ?", id)
	return err
}

func ClearHistory() error {
	_, err := db.Exec("DELETE FROM history")
	return err
}

// DeleteHistoryBatch deletes multiple history entries by their IDs.
func DeleteHistoryBatch(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "DELETE FROM history WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := db.Exec(query, args...)
	return err
}

// DeleteHistoryOlderThan deletes entries with created_at strictly before
// the entry with the given ID.
func DeleteHistoryOlderThan(id int64) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM history WHERE created_at < (SELECT created_at FROM history WHERE id = ?)`, id,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteHistoryDuplicates removes older duplicates, keeping only the newest
// entry for each method+URL combination.
func DeleteHistoryDuplicates() (int64, error) {
	result, err := db.Exec(`
		DELETE FROM history WHERE id NOT IN (
			SELECT MAX(id) FROM history GROUP BY method, url
		)`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func HeadersFromJSON(s string) map[string]string {
	var m map[string]string
	_ = json.Unmarshal([]byte(s), &m) // malformed headers default to empty
	if m == nil {
		m = make(map[string]string)
	}
	return m
}
