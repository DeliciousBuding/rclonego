package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/api"
	"github.com/DeliciousBuding/rclonego/internal/core"
)

func testApp(t *testing.T) *core.App {
	t.Helper()
	dir := t.TempDir()
	store, err := core.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return core.NewApp(store, "", dir)
}

func testHandler(t *testing.T, app *core.App) http.Handler {
	t.Helper()
	token := strings.Repeat("t", 64)
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RCLONEGO_TOKEN_FILE", path)
	server, err := api.New(app, token)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler()
}
