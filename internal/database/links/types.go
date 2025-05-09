package links

import (
	"database/sql"
	"time"
)

type Store struct {
	db *sql.DB
}

type UploadLink struct {
	Id            int
	RemainingUses int
	Token         string
	Dir           string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	LastUsedAt    *time.Time
	Url           string
}

type DownloadLink struct {
	Id            int
	RemainingUses int
	Token         string
	Dir           string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	LastUsedAt    *time.Time
	Url           string
}
