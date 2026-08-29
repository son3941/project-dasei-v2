package handler

import (
	"database/sql"
	"log/slog"
	"os"
	"strings"
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
			db.Close()
			db = nil
			return
		}

		slog.Info("Ping OK")
		slog.Info("creating tables")

		_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS memories (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
`)
		if err != nil {
			slog.Error("CREATE memories TABLE failed",
				slog.String("error", err.Error()),
			)
			return
		}

		_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
`)
		if err != nil {
			slog.Error("CREATE posts TABLE failed",
				slog.String("error", err.Error()),
			)
			return
		}

		slog.Info("tables created")
	})

	if err != nil {
		return err
	}

	if db == nil {
		return sql.ErrConnDone
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS learned_pairs (
    id SERIAL PRIMARY KEY,
    pair_key TEXT NOT NULL,
    pair_value TEXT NOT NULL,
    UNIQUE(pair_key, pair_value)
)
`)
	if err != nil {
		slog.Error("CREATE learned_pairs TABLE failed",
			slog.String("error", err.Error()),
		)
		return err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS nicknames (
    member_name TEXT PRIMARY KEY,
    nickname TEXT NOT NULL
)
`)
	if err != nil {
		slog.Error("CREATE nicknames TABLE failed",
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}
func SaveNickname(name, nickname string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`
INSERT INTO nicknames(member_name, nickname)
VALUES ($1, $2)
ON CONFLICT (member_name)
DO UPDATE SET
    nickname = EXCLUDED.nickname
`,
		name,
		nickname,
	)

	return err
}

func LoadNickname(name string) (string, error) {
	if err := initDB(); err != nil {
		return "", err
	}

	var nickname string

	err := db.QueryRow(`
SELECT nickname
FROM nicknames
WHERE member_name = $1
`,
		name,
	).Scan(&nickname)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(nickname), nil
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

	result, err := db.Exec(`
DELETE FROM memories;
DELETE FROM posts;
DELETE FROM nicknames;
`)
	rows, _ := result.RowsAffected()
	slog.Info("ClearMemories",
		slog.Int64("rows", rows),
	)
	if err != nil {
		return err
	}

	memoryMu.Lock()
	memories = make(map[string]Memory)
	memoryMu.Unlock()

	learnedWordsMu.Lock()
	learnedPairs = nil
	learnedPhrases = nil
	learnedWordsMu.Unlock()

	return nil
}
func SaveLearnedPair(key, value string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(
		`DELETE FROM learned_pairs WHERE pair_key = $1`,
		key,
	)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO learned_pairs (pair_key, pair_value)
         VALUES ($1, $2)`,
		key, value,
	)
	return err
}

func LoadLearnedPairs() ([]LearnedPair, error) {
	if err := initDB(); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
        SELECT pair_key, pair_value
        FROM learned_pairs
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pairs := make([]LearnedPair, 0)

	for rows.Next() {
		var key, value string

		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		pairs = append(pairs, LearnedPair{
			Key:   key,
			Value: value,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}

func DeleteLearnedPairsByName(name string) error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`
        DELETE FROM learned_pairs
        WHERE pair_key = ? OR pair_value = ?
    `, name, name)

	return err
}

func ClearLearnedPairs() error {
	if err := initDB(); err != nil {
		return err
	}

	_, err := db.Exec(`DELETE FROM learned_pairs`)

	return err
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
