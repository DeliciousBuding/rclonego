// Package persistence provides transactional state and event storage.
package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

type DB struct{ db *sqlite.DB }

type Record struct {
	ID, Parent, Status string
	Position           int
	Payload            []byte
}

type Event struct {
	Subject string
	Kind    string
}

func Open(path string) (*DB, error) {
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	result := &DB{db: db}
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=FULL",
		"CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS workflows (id TEXT PRIMARY KEY, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS steps (id TEXT PRIMARY KEY, run_id TEXT NOT NULL, status TEXT NOT NULL, position INTEGER NOT NULL, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS attempts (id TEXT PRIMARY KEY, step_id TEXT NOT NULL, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS events (seq INTEGER PRIMARY KEY AUTOINCREMENT, occurred_at TEXT NOT NULL, subject TEXT NOT NULL, kind TEXT NOT NULL)",
	} {
		if _, err := db.Execute(context.Background(), statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return result, nil
}

func (d *DB) Close() error { return d.db.Close() }

func (d *DB) Load() (map[string][]json.RawMessage, error) {
	result := map[string][]json.RawMessage{}
	for _, table := range []string{"settings", "workflows", "runs", "steps", "attempts"} {
		query := "SELECT payload FROM " + table
		if table == "steps" {
			query += " ORDER BY position, id"
		}
		rows, err := d.db.Query(context.Background(), query)
		if err != nil {
			return nil, err
		}
		for _, row := range rows.Values {
			raw, ok := row[0].([]byte)
			if !ok {
				return nil, fmt.Errorf("invalid persisted %s payload", table)
			}
			result[table] = append(result[table], json.RawMessage(raw))
		}
	}
	return result, nil
}

func (d *DB) Save(settings []byte, records map[string][]Record, events []Event) error {
	statements := []sqlite.Statement{{
		SQL:  "INSERT INTO settings(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload",
		Args: []any{settings},
	}}
	for _, table := range []string{"workflows", "runs", "steps", "attempts"} {
		items := records[table]
		if table == "workflows" || table == "runs" || table == "steps" {
			// The in-memory snapshot is authoritative; attempts remain historical.
			statements = append(statements, sqlite.Statement{SQL: "DELETE FROM " + table})
		}
		for _, record := range items {
			var statement sqlite.Statement
			switch table {
			case "workflows":
				statement = sqlite.Statement{SQL: "INSERT INTO workflows VALUES(?,?)", Args: []any{record.ID, record.Payload}}
			case "runs":
				statement = sqlite.Statement{SQL: "INSERT INTO runs VALUES(?,?,?)", Args: []any{record.ID, record.Parent, record.Payload}}
			case "steps":
				statement = sqlite.Statement{SQL: "INSERT INTO steps VALUES(?,?,?,?,?)",
					Args: []any{record.ID, record.Parent, record.Status, record.Position, record.Payload}}
			case "attempts":
				statement = sqlite.Statement{SQL: "INSERT INTO attempts VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload",
					Args: []any{record.ID, record.Parent, record.Payload}}
			}
			statements = append(statements, statement)
		}
	}
	for _, event := range events {
		statements = append(statements, sqlite.Statement{
			SQL:  "INSERT INTO events(occurred_at,subject,kind) VALUES(?,?,?)",
			Args: []any{time.Now().UTC().Format(time.RFC3339Nano), event.Subject, event.Kind},
		})
	}
	_, err := d.db.Transaction(context.Background(), statements)
	return err
}
