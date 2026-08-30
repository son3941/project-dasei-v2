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

		db, err = sql.Open("postgres", dsn)
		if err != nil {
			return
		}

		if err = db.Ping(); err != nil {
			db.Close()
			db = nil
			return
		}

		// DB移行履歴。
		_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`)
		if err != nil {
			return
		}

		var migrated bool

		err = db.QueryRow(`
SELECT EXISTS (
    SELECT 1
    FROM schema_migrations
    WHERE version = 'community_memory_v1'
)
`).Scan(&migrated)

		if err != nil {
			return
		}
		if !migrated {
			slog.Info("migrating to community memory v1")

			// B案：
			// 旧共有記憶はここで一度だけリセットする。
			_, err = db.Exec(`
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS learned_pairs;
DROP TABLE IF EXISTS nicknames;
`)
			if err != nil {
				return
			}

			_, err = db.Exec(`
CREATE TABLE memories (
    community_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    last_used TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (community_id, key)
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    community_id TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE learned_pairs (
    id SERIAL PRIMARY KEY,
    community_id TEXT NOT NULL,
    pair_key TEXT NOT NULL,
    pair_value TEXT NOT NULL,
    UNIQUE (community_id, pair_key, pair_value)
);
`)
			if err != nil {
				return
			}
			_, err = db.Exec(`
CREATE TABLE nicknames (
    community_id TEXT NOT NULL,
    member_name TEXT NOT NULL,
    nickname TEXT NOT NULL,
    PRIMARY KEY (community_id, member_name)
);

INSERT INTO schema_migrations(version)
VALUES ('community_memory_v1');
`)
			if err != nil {
				return
			}

			slog.Info("community memory v1 migration complete")
		}
	})

	if err != nil {
		return err
	}

	if db == nil {
		return sql.ErrConnDone
	}

	return nil
}
func SaveNickname(
	communityID string,
	memberName string,
	nickname string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)
	memberName = strings.TrimSpace(memberName)
	nickname = strings.TrimSpace(nickname)

	if communityID == "" || memberName == "" || nickname == "" {
		return nil
	}

	_, err := db.Exec(`
INSERT INTO nicknames (
    community_id,
    member_name,
    nickname
)
VALUES ($1, $2, $3)
ON CONFLICT (
    community_id,
    member_name
)
DO UPDATE SET
    nickname = EXCLUDED.nickname
`,
		communityID,
		memberName,
		nickname,
	)

	return err
}
func LoadNickname(
	communityID string,
	memberName string,
) (string, error) {
	if err := initDB(); err != nil {
		return "", err
	}

	communityID = strings.TrimSpace(communityID)
	memberName = strings.TrimSpace(memberName)

	if communityID == "" || memberName == "" {
		return "", nil
	}

	var nickname string

	err := db.QueryRow(`
SELECT nickname
FROM nicknames
WHERE community_id = $1
  AND member_name = $2
`,
		communityID,
		memberName,
	).Scan(&nickname)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(nickname), nil
}
func SaveMemory(communityID, key, value string) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	if communityID == "" || key == "" || value == "" {
		return nil
	}

	_, err := db.Exec(`
INSERT INTO memories(community_id, key, value, last_used)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
ON CONFLICT (community_id, key)
DO UPDATE SET
    value = EXCLUDED.value,
    last_used = CURRENT_TIMESTAMP
`,
		communityID,
		key,
		value,
	)

	return err
}
func LoadMemories(communityID string) (map[string]string, error) {
	if err := initDB(); err != nil {
		return nil, err
	}

	communityID = strings.TrimSpace(communityID)

	result := make(map[string]string)

	if communityID == "" {
		return result, nil
	}

	rows, err := db.Query(`
SELECT key, value
FROM memories
WHERE community_id = $1
`,
		communityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string

		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		result[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
func ClearMemories(
	communityID string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`
DELETE FROM memories
WHERE community_id = $1
`, communityID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
DELETE FROM posts
WHERE community_id = $1
`, communityID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
DELETE FROM learned_pairs
WHERE community_id = $1
`, communityID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
DELETE FROM nicknames
WHERE community_id = $1
`, communityID); err != nil {
		return err
	}

	return tx.Commit()
}
func SaveLearnedPair(
	communityID string,
	key string,
	value string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	if communityID == "" || key == "" || value == "" {
		return nil
	}

	_, err := db.Exec(`
INSERT INTO learned_pairs (
    community_id,
    pair_key,
    pair_value
)
VALUES ($1, $2, $3)
ON CONFLICT (
    community_id,
    pair_key,
    pair_value
)
DO NOTHING
`,
		communityID,
		key,
		value,
	)

	return err
}
func LoadLearnedPairs(
	communityID string,
) ([]LearnedPair, error) {
	if err := initDB(); err != nil {
		return nil, err
	}

	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return []LearnedPair{}, nil
	}

	rows, err := db.Query(`
SELECT pair_key, pair_value
FROM learned_pairs
WHERE community_id = $1
`,
		communityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pairs := make([]LearnedPair, 0)

	for rows.Next() {
		var key, value string

		if err := rows.Scan(
			&key,
			&value,
		); err != nil {
			return nil, err
		}

		pairs = append(
			pairs,
			LearnedPair{
				Key:   key,
				Value: value,
			},
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}
func DeleteLearnedPairsByName(
	communityID string,
	name string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)
	name = strings.TrimSpace(name)

	if communityID == "" || name == "" {
		return nil
	}

	_, err := db.Exec(`
DELETE FROM learned_pairs
WHERE community_id = $1
  AND (pair_key = $2 OR pair_value = $2)
`,
		communityID,
		name,
	)

	return err
}
func ClearLearnedPairs(
	communityID string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return nil
	}

	_, err := db.Exec(`
DELETE FROM learned_pairs
WHERE community_id = $1
`,
		communityID,
	)

	return err
}
func SavePost(
	communityID string,
	text string,
) error {
	if err := initDB(); err != nil {
		return err
	}

	communityID = strings.TrimSpace(communityID)
	text = strings.TrimSpace(text)

	if communityID == "" || text == "" {
		return nil
	}

	_, err := db.Exec(`
INSERT INTO posts (
    community_id,
    text
)
VALUES ($1, $2)
`,
		communityID,
		text,
	)

	return err
}
func LoadRandomPost(
	communityID string,
) (string, error) {
	if err := initDB(); err != nil {
		return "", err
	}

	communityID = strings.TrimSpace(communityID)

	if communityID == "" {
		return "", nil
	}

	var text string

	err := db.QueryRow(`
SELECT text
FROM posts
WHERE community_id = $1
ORDER BY RANDOM()
LIMIT 1
`,
		communityID,
	).Scan(&text)

	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(text), nil
}
