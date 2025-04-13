package sessions

import (
	"database/sql"
	"time"
)

type Store struct {
	db *sql.DB
}

type Session struct {
	Id        string
	CreatedAt time.Time
	ExpiresAt time.Time
}
