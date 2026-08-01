package handler

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	db         *sql.DB
	dbInitOnce sync.Once
)
