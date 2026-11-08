//go:build !darwin

package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguagePickerRestoresExistingMyGoPreference(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "")
	app := testApp(t)
	directory := app.DataDirectory()
	if got := New(app, "").languageOverride; got != "" {
		t.Fatalf("new profile must follow system: %q", got)
	}
	path := filepath.Join(directory, "locale")
	if err := os.WriteFile(path, []byte("en\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := New(app, "").languageOverride; got != "en" {
		t.Fatalf("existing explicit language was lost: %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := New(app, "").languageOverride; got != "" {
		t.Fatalf("reset preference no longer follows system: %q", got)
	}
	if err := os.WriteFile(path, []byte("not a locale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := savedLanguagePreference(directory); got != "" {
		t.Fatalf("invalid override bypassed MyGo normalization: %q", got)
	}
}

func TestDevelopmentLanguageOverrideDoesNotAlterSavedPreference(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "locale")
	if err := os.WriteFile(path, []byte("en\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	if got := savedLanguagePreference(directory); got != "zh-Hans" {
		t.Fatalf("development language was ignored: %q", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "en\n" {
		t.Fatalf("development override changed the saved preference: %q / %v", raw, err)
	}
}
