package persistence

import (
	"path/filepath"
	"testing"
)

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
