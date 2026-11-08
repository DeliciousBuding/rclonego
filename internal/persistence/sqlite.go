// Package persistence provides transactional state and event storage.
package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

type DB struct {
	db       *sqlite.DB
	cached   map[string]map[string][]byte
	settings []byte
}

type Record struct {
	ID, Parent, Status string
	Position           int
	Payload            []byte
}

type Event struct {
	Seq        int64  `json:"seq"`
	OccurredAt string `json:"occurred_at"`
	Subject    string `json:"subject"`
	Kind       string `json:"kind"`
}

const schemaVersion = 1

func Open(path string) (*DB, error) {
	if err := prepareNative(); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	// Inspect the version before changing pragmas or creating any tables.
	version, err := db.Query(context.Background(), "PRAGMA user_version")
	if err != nil || len(version.Values) != 1 {
		db.Close()
		if err == nil {
			err = fmt.Errorf("invalid database schema version")
		}
		return nil, err
	}
	current, ok := version.Values[0][0].(int64)
	if !ok || current < 0 || current > schemaVersion {
		db.Close()
		return nil, fmt.Errorf("unsupported database schema %v; use a compatible RcloneGo version", version.Values[0][0])
	}
	result := &DB{db: db, cached: map[string]map[string][]byte{}}
	statements := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=FULL",
		"CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS workflows (id TEXT PRIMARY KEY, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS steps (id TEXT PRIMARY KEY, run_id TEXT NOT NULL, status TEXT NOT NULL, position INTEGER NOT NULL, payload BLOB NOT NULL)",
		"CREATE INDEX IF NOT EXISTS steps_idempotency ON steps(json_extract(CAST(payload AS TEXT),'$.idempotency_key'))",
		"CREATE TABLE IF NOT EXISTS attempts (id TEXT PRIMARY KEY, step_id TEXT NOT NULL, payload BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS events (seq INTEGER PRIMARY KEY AUTOINCREMENT, occurred_at TEXT NOT NULL, subject TEXT NOT NULL, kind TEXT NOT NULL)",
	}
	for _, statement := range statements {
		if _, err := db.Execute(context.Background(), statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	if current == 0 {
		if _, err := db.Execute(context.Background(), "PRAGMA user_version=1"); err != nil {
			db.Close()
			return nil, err
		}
	}
	return result, nil
}

func (d *DB) Check() error {
	_, err := d.db.Query(context.Background(), "SELECT 1")
	return err
}

func (d *DB) JobByKey(key string) (json.RawMessage, error) {
	rows, err := d.db.Query(context.Background(), "SELECT payload FROM steps WHERE json_extract(CAST(payload AS TEXT),'$.idempotency_key')=? LIMIT 1", key)
	if err != nil {
		return nil, err
	}
	if len(rows.Values) == 0 {
		return nil, nil
	}
	raw, ok := rows.Values[0][0].([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid job payload")
	}
	return json.RawMessage(raw), nil
}

func (d *DB) Close() error { return d.db.Close() }

// Forget removes an evicted row from the snapshot cache, not from SQLite.
// Intentional deletion still follows Save's authoritative tracked rows.
func (d *DB) Forget(table, id string) { delete(d.cached[table], id) }

func (d *DB) Load() (map[string][]json.RawMessage, error) {
	result := map[string][]json.RawMessage{}
	for _, table := range []string{"settings", "workflows", "runs", "steps", "attempts"} {
		query := "SELECT id,payload FROM " + table
		d.cached[table] = map[string][]byte{}
		switch table {
		case "runs":
			query += " WHERE id IN (SELECT id FROM runs ORDER BY rowid DESC LIMIT 200) OR id IN (SELECT run_id FROM steps WHERE status IN ('pending','running','paused','interrupted'))"
		case "steps":
			query += " WHERE status IN ('pending','running','paused','interrupted') OR run_id IN (SELECT id FROM runs ORDER BY rowid DESC LIMIT 200) OR id IN (SELECT id FROM steps WHERE run_id='' ORDER BY rowid DESC LIMIT 500)"
			query += " ORDER BY position,id"
		case "attempts":
			query += " WHERE step_id IN (SELECT id FROM steps WHERE status IN ('pending','running','paused','interrupted') OR run_id IN (SELECT id FROM runs ORDER BY rowid DESC LIMIT 200) OR id IN (SELECT id FROM steps WHERE run_id='' ORDER BY rowid DESC LIMIT 500))"
		}
		rows, err := d.db.Query(context.Background(), query)
		if err != nil {
			return nil, err
		}
		for _, row := range rows.Values {
			raw, ok := row[1].([]byte)
			if !ok {
				return nil, fmt.Errorf("invalid persisted %s payload", table)
			}
			result[table] = append(result[table], json.RawMessage(raw))
			if table == "settings" {
				d.settings = append([]byte(nil), raw...)
			} else {
				id, _ := row[0].(string)
				d.cached[table][id] = append([]byte(nil), raw...)
			}
		}
	}
	return result, nil
}

func (d *DB) RunRecords(id string) (map[string][]json.RawMessage, error) {
	result := map[string][]json.RawMessage{}
	for _, table := range []string{"runs", "steps", "attempts"} {
		query := "SELECT payload FROM " + table
		switch table {
		case "runs":
			query += " WHERE id=?"
		case "steps":
			query += " WHERE run_id=? ORDER BY position,id"
		case "attempts":
			query += " WHERE step_id IN (SELECT id FROM steps WHERE run_id=?)"
		}
		rows, err := d.db.Query(context.Background(), query, id)
		if err != nil {
			return nil, err
		}
		for _, row := range rows.Values {
			raw, ok := row[0].([]byte)
			if !ok {
				return nil, fmt.Errorf("invalid run record")
			}
			result[table] = append(result[table], json.RawMessage(raw))
		}
	}
	return result, nil
}

func (d *DB) Runs(offset, limit int) ([]json.RawMessage, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset must be nonnegative")
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := d.db.Query(context.Background(), "SELECT payload FROM runs ORDER BY rowid DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	result := []json.RawMessage{}
	for _, row := range rows.Values {
		raw, ok := row[0].([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid run record")
		}
		result = append(result, json.RawMessage(raw))
	}
	return result, nil
}

func (d *DB) Save(settings []byte, records map[string][]Record, events []Event) error {
	var statements []sqlite.Statement
	if !bytes.Equal(settings, d.settings) {
		statements = append(statements, sqlite.Statement{
			SQL:  "INSERT INTO settings(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload",
			Args: []any{settings},
		})
	}
	next := map[string]map[string][]byte{}
	for _, table := range []string{"workflows", "runs", "steps", "attempts"} {
		items := records[table]
		next[table] = map[string][]byte{}
		for _, record := range items {
			next[table][record.ID] = record.Payload
			if bytes.Equal(d.cached[table][record.ID], record.Payload) {
				continue
			}
			var statement sqlite.Statement
			switch table {
			case "workflows":
				statement = sqlite.Statement{SQL: "INSERT INTO workflows VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", Args: []any{record.ID, record.Payload}}
			case "runs":
				statement = sqlite.Statement{SQL: "INSERT INTO runs VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET workflow_id=excluded.workflow_id,payload=excluded.payload", Args: []any{record.ID, record.Parent, record.Payload}}
			case "steps":
				statement = sqlite.Statement{SQL: "INSERT INTO steps VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id,status=excluded.status,position=excluded.position,payload=excluded.payload",
					Args: []any{record.ID, record.Parent, record.Status, record.Position, record.Payload}}
			case "attempts":
				statement = sqlite.Statement{SQL: "INSERT INTO attempts VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload",
					Args: []any{record.ID, record.Parent, record.Payload}}
			}
			statements = append(statements, statement)
		}
		for id := range d.cached[table] {
			if _, exists := next[table][id]; !exists {
				statements = append(statements, sqlite.Statement{SQL: "DELETE FROM " + table + " WHERE id=?", Args: []any{id}})
			}
		}
	}
	for _, event := range events {
		statements = append(statements, sqlite.Statement{
			SQL:  "INSERT INTO events(occurred_at,subject,kind) VALUES(?,?,?)",
			Args: []any{time.Now().UTC().Format(time.RFC3339Nano), event.Subject, event.Kind},
		})
	}
	if len(events) > 0 {
		statements = append(statements, sqlite.Statement{SQL: "DELETE FROM events WHERE seq <= (SELECT COALESCE(MAX(seq),0)-10000 FROM events)"})
	}
	if len(statements) == 0 {
		return nil
	}
	_, err := d.db.Transaction(context.Background(), statements)
	if err == nil {
		d.cached, d.settings = next, append([]byte(nil), settings...)
	}
	return err
}

func (d *DB) Events(after int64, limit int) ([]Event, error) {
	if after < 0 {
		return nil, fmt.Errorf("after must be nonnegative")
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := d.db.Query(context.Background(), "SELECT seq,occurred_at,subject,kind FROM events WHERE seq>? ORDER BY seq LIMIT ?", after, limit)
	if err != nil {
		return nil, err
	}
	result := []Event{}
	for _, row := range rows.Values {
		seq, _ := row[0].(int64)
		at, _ := row[1].(string)
		subject, _ := row[2].(string)
		kind, _ := row[3].(string)
		result = append(result, Event{Seq: seq, OccurredAt: at, Subject: subject, Kind: kind})
	}
	return result, nil
}
