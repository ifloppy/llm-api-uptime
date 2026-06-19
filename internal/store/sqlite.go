package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			base_url    TEXT NOT NULL,
			api_key     TEXT NOT NULL,
			api_type    TEXT NOT NULL CHECK(api_type IN ('openai', 'anthropic')),
			enabled     BOOLEAN DEFAULT 1,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS probes (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
			model       TEXT NOT NULL,
			enabled     BOOLEAN DEFAULT 1,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider_id, model)
		)`,
		`CREATE TABLE IF NOT EXISTS results (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			probe_id      INTEGER NOT NULL REFERENCES probes(id) ON DELETE CASCADE,
			status        TEXT NOT NULL CHECK(status IN ('success', 'error', 'timeout', 'empty_response')),
			status_code   INTEGER,
			latency_ms    INTEGER,
			error_code    TEXT,
			error_message TEXT,
			request_id    TEXT,
			raw_error     TEXT,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_results_probe_created ON results(probe_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_results_created ON results(created_at)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w\nSQL: %s", err, m)
		}
	}

	return nil
}

func (s *Store) Cleanup(retentionDays int) error {
	_, err := s.db.Exec("DELETE FROM results WHERE created_at < datetime('now', ?)", fmt.Sprintf("-%d days", retentionDays))
	return err
}
