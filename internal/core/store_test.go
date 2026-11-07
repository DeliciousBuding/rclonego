package core

import "testing"

func TestDeletedMappingsAndAppearanceSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Update(func(state *State) error { state.Mounts = nil; state.Appearance = "dark"; return nil }); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	state := reopened.Snapshot()
	if len(state.Mounts) != 0 || state.Appearance != "dark" {
		t.Fatalf("settings changed on restart: %+v", state)
	}
}
