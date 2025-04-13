package links

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func NewLinkStore(db *sql.DB) (*Store, error) {
	ls := &Store{db: db}
	if err := ls.initialize(); err != nil {
		return nil, err
	}
	return ls, nil
}

func (ls *Store) initialize() error {
	_, err := ls.db.Exec(`
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
	return err
}

func (ls *Store) ListActive() ([]Link, error) {
	return ls.List(true)
}

func (ls *Store) List(active bool) ([]Link, error) {
	links := make([]Link, 0)
	rows, err := ls.db.Query("SELECT remaining_uses, token, dir, expires_at, created_at, last_used_at FROM upload_links")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch upload links: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.RemainingUses, &link.Token, &link.Dir, &link.ExpiresAt, &link.CreatedAt, &link.LastUsedAt); err != nil {
			return nil, fmt.Errorf("failed to scan upload link: %v", err)
		}
		if active && (link.RemainingUses <= 0 || link.ExpiresAt.Before(time.Now())) {
			continue
		}
		link.Url = fmt.Sprintf("/upload/%s/", link.Token)
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over upload links: %v", err)
	}
	return links, nil
}

func (ls *Store) Create(token, dir string, expiresAt time.Time, remainingUses int) error {
	_, err := ls.db.Exec(
		"INSERT INTO upload_links (token, dir, expires_at, remaining_uses, created_at) VALUES (?, ?, ?, ?, ?)",
		token,
		dir,
		expiresAt,
		remainingUses,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to create upload link: %v", err)
	}
	return nil
}

func (ls *Store) Deactivate(token string) error {
	return ls.Update(token, 0, time.Now())
}

func (ls *Store) Delete(token string) error {
	_, err := ls.db.Exec(
		"DELETE FROM upload_links WHERE token = ?",
		token,
	)
	if err != nil {
		return fmt.Errorf("failed to delete upload link: %v", err)
	}
	return nil
}

func (ls *Store) Get(token string) (*Link, error) {
	var link Link
	err := ls.db.QueryRow(
		"SELECT remaining_uses, token, dir, expires_at, created_at, last_used_at FROM upload_links WHERE token = ?",
		token,
	).Scan(&link.RemainingUses, &link.Token, &link.Dir, &link.ExpiresAt, &link.CreatedAt, &link.LastUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("upload link not found")
		}
		return nil, fmt.Errorf("failed to fetch upload link: %v", err)
	}
	link.Url = fmt.Sprintf("/upload/%s/", link.Token)
	return &link, nil
}

func (ls *Store) Update(token string, remainingUses int, lastUsed time.Time) error {
	_, err := ls.db.Exec(
		"UPDATE upload_links SET remaining_uses = ?, last_used_at = ? WHERE token = ?",
		remainingUses,
		lastUsed,
		token,
	)
	if err != nil {
		return fmt.Errorf("failed to update upload link: %v", err)
	}
	return nil
}
