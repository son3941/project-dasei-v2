package handler

import (
	"database/sql"
	"os"
	"sync"

	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	dbInitOnce sync.Once
)

func initDB() error {
	var err error

	dbInitOnce.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			err = sql.ErrConnDone
			return
		}

		db, err = sql.Open("postgres", dsn)
		if err != nil {
			return
		}

		if err = db.Ping(); err != nil {
			return
		}

		_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS memories (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
`)
	})

	return err
}

func SaveMemory(key, value string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`
INSERT INTO memories(key, value, last_used)
VALUES ($1, $2, CURRENT_TIMESTAMP)
ON CONFLICT (key)
DO UPDATE SET
    value = EXCLUDED.value,
    last_used = CURRENT_TIMESTAMP
`,
		key,
		value,
	)

	return err
}

func LoadMemories() (map[string]string, error) {
	if err := initDB(); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT key, value
FROM memories
`)
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
