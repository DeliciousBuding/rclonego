package persistence

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

func TestFutureDatabaseSchemaIsRejectedWithoutChangingTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	if err := prepareNative(); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Execute(context.Background(), "PRAGMA user_version=999"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if upgraded, err := Open(path); err == nil {
		upgraded.Close()
		t.Fatal("future database schema was silently accepted")
	}
	db, err = sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil || len(rows.Values) != 0 {
		t.Fatal("opening a future schema created application tables")
	}
	version, err := db.Query(context.Background(), "PRAGMA user_version")
	if err != nil || version.Values[0][0] != int64(999) {
		t.Fatal("future schema version was changed")
	}
}

func TestIncrementalSaveReopenAndEventCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := []byte(`{"version":1}`)
	records := map[string][]Record{"steps": {{ID: "step", Parent: "run", Status: "pending", Payload: []byte(`{"id":"step","status":"pending"}`)}}}
	if err := db.Save(settings, records, []Event{{Subject: "step", Kind: "pending"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Save(settings, records, nil); err != nil {
		t.Fatal(err)
	}
	events, err := db.Events(0, 100)
	if err != nil || len(events) != 1 || events[0].Seq <= 0 {
		t.Fatalf("event sequence: %+v %v", events, err)
	}
	next, err := db.Events(events[0].Seq, 100)
	if err != nil || len(next) != 0 {
		t.Fatal("cursor returned old events")
	}
	db.Close()
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	loaded, err := db.Load()
	if err != nil || len(loaded["steps"]) != 1 {
		t.Fatalf("reopen: %v", err)
	}
	records["steps"] = nil
	if err := db.Save(settings, records, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.Load()
	if err != nil || len(loaded["steps"]) != 0 {
		t.Fatal("deleted record survived incremental save")
	}
}
