package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1) // SQLite WAL works best single-writer
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	db.migrateUpgrade()
	return db, nil
}

// Close closes the underlying connection.
func (db *DB) Close() error { return db.conn.Close() }

// migrateUpgrade runs ALTER TABLE statements for columns added after the initial
// schema. Errors are intentionally ignored so that the statements are
// effectively idempotent (SQLite returns "duplicate column name" on re-runs).
func (db *DB) migrateUpgrade() {
	alters := []string{
		`ALTER TABLE environments ADD COLUMN priority INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE notes ADD COLUMN template_id INTEGER REFERENCES templates(id) ON DELETE CASCADE`,
		`ALTER TABLE templates ADD COLUMN service_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE templates ADD COLUMN path TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alters {
		db.conn.Exec(q) //nolint:errcheck — duplicate column is expected on re-runs
	}
}

func (db *DB) migrate() error {
	queries := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`CREATE TABLE IF NOT EXISTS users (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT    NOT NULL UNIQUE,
			password TEXT    NOT NULL,
			role     TEXT    NOT NULL DEFAULT 'standard'
		);`,
		`CREATE TABLE IF NOT EXISTS environments (
			id    INTEGER PRIMARY KEY AUTOINCREMENT,
			name  TEXT NOT NULL UNIQUE,
			color TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS variables (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL UNIQUE,
			is_password INTEGER NOT NULL DEFAULT 0,
			values_json TEXT    NOT NULL DEFAULT '{}'
		);`,
		`CREATE TABLE IF NOT EXISTS technologies (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			name              TEXT NOT NULL UNIQUE,
			method            TEXT NOT NULL DEFAULT 'GET',
			url               TEXT NOT NULL DEFAULT '',
			body              TEXT NOT NULL DEFAULT '',
			headers_json      TEXT NOT NULL DEFAULT '[]',
			custom_values_json TEXT NOT NULL DEFAULT '[]'
		);`,
		`CREATE TABLE IF NOT EXISTS templates (
			id                   INTEGER PRIMARY KEY AUTOINCREMENT,
			name                 TEXT    NOT NULL UNIQUE,
			method               TEXT    NOT NULL DEFAULT 'GET',
			url                  TEXT    NOT NULL DEFAULT '',
			body                 TEXT    NOT NULL DEFAULT '',
			headers_json         TEXT    NOT NULL DEFAULT '[]',
			custom_values_json   TEXT    NOT NULL DEFAULT '[]',
			restricted_execution INTEGER NOT NULL DEFAULT 0,
			technology_id        INTEGER REFERENCES technologies(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS notes (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			title          TEXT    NOT NULL,
			body           TEXT    NOT NULL DEFAULT '',
			is_private     INTEGER NOT NULL DEFAULT 0,
			owner_username TEXT    NOT NULL,
			environment_id INTEGER REFERENCES environments(id) ON DELETE SET NULL,
			created_at     DATETIME NOT NULL DEFAULT (datetime('now'))
		);`,
	}
	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q, err)
		}
	}
	if err := db.seed(); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}
