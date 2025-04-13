package sessions

import (
	"database/sql"
	"time"
)

func NewSessionStore(db *sql.DB) (*Store, error) {
	ss := &Store{db: db}
	if err := ss.initialize(); err != nil {
		return nil, err
	}
	return ss, nil
}

func (ls *Store) initialize() error {
	_, err := ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL
		);
	`)
	return err
}

func (ss *Store) Create(sessionId string, createdAt, expiresAt time.Time) error {
	_, err := ss.db.Exec(`
		INSERT INTO sessions (id, created_at, expires_at)
		VALUES (?, ?, ?)
	`, sessionId, createdAt, expiresAt)
	return err
}

func (ss *Store) Get(sessionId string) (*Session, error) {
	var session Session
	err := ss.db.QueryRow(`
		SELECT id, created_at, expires_at
		FROM sessions
		WHERE id = ?
	`, sessionId).Scan(&session.Id, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (ss *Store) Delete(sessionId string) error {
	_, err := ss.db.Exec(`
		DELETE FROM sessions
		WHERE id = ?
	`, sessionId)
	return err
}
