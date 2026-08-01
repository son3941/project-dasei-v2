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

func initDB() error {
	var err error

	dbInitOnce.Do(func() {
		db, err = sql.Open("sqlite", "dasei.db")
		if err != nil {
			return
		}

		_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			last_used DATETIME
		)
		`)
	})

	return err
}

func SaveMemory(key, value string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(
		`INSERT OR REPLACE INTO memories(key, value, last_used)
		 VALUES (?, ?, CURRENT_TIMESTAMP)`,
		key,
		value,
	)

	return err
}
func LoadMemories() (map[string]string, error) {
	if err := initDB(); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT key, value FROM memories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := make(map[string]string)

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		memories[key] = value
	}

	return memories, nil
}
