package handler

import (
	"database/sql"
	"log/slog"
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

		slog.Info("DATABASE_URL",
			slog.Bool("exists", dsn != ""),
		)

		if dsn == "" {
			err = sql.ErrConnDone
			return
		}
		slog.Info("opening postgres")
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			slog.Error("sql.Open failed",
				slog.String("error", err.Error()),
			)
			return
		}

		slog.Info("sql.Open OK")

		if err = db.Ping(); err != nil {
			slog.Error("Ping failed",
				slog.String("error", err.Error()),
			)
			return
		}
		slog.Info("Ping OK")
		slog.Info("creating table")

		_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS memories (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
`)

		if err != nil {
			slog.Error("CREATE TABLE failed",
				slog.String("error", err.Error()),
			)
			return
		}

		slog.Info("table created")
	})
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
`)
	if err != nil {
		slog.Error("CREATE POSTS TABLE failed",
			slog.String("error", err.Error()),
		)
		return err
	}

	slog.Info("posts table created")
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
func ClearMemories() error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`
DELETE FROM memories;
DELETE FROM posts;
`)
	if err != nil {
		return err
	}

	memoryMu.Lock()
	memories = make(map[string]Memory)
	memoryMu.Unlock()

	return nil
}

func SavePost(text string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`
INSERT INTO posts(text)
VALUES ($1)
`, text)

	return err
}
func LoadRandomPost() (string, error) {
	if err := initDB(); err != nil {
		return "", err
	}

	var text string

	err := db.QueryRow(`
SELECT text
FROM posts
ORDER BY RANDOM()
LIMIT 1
`).Scan(&text)

	return text, err
}
