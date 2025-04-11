package main

import (
	"database/sql"
	"fmt"
)

func initDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?cache=shared&mode=rwc&_journal_mode=WAL", dbPath))
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS upload_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP NOT NULL,
			last_used_at TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			dir TEXT NOT NULL,
			remaining_uses INTEGER NOT NULL DEFAULT 1
		);
	`)

	return db, err
}
