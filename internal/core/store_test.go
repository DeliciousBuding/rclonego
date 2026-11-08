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

func TestAppearanceFollowsSystemForNewAndMissingPreferences(t *testing.T) {
	for _, existing := range []string{"new", "", "invalid", "light", "dark", "system"} {
		t.Run(existing, func(t *testing.T) {
			directory := t.TempDir()
			store, err := OpenStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			if got := store.Snapshot().Appearance; got != "system" {
				t.Fatalf("new profile appearance = %q", got)
			}
			if existing != "new" {
				if err := store.Update(func(state *State) error {
					state.Appearance = existing
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			expected := existing
			if !ValidAppearance(expected) {
				expected = "system"
			}
			if got := reopened.Snapshot().Appearance; got != expected {
				t.Fatalf("reopened appearance = %q; want %q", got, expected)
			}
		})
	}
}
