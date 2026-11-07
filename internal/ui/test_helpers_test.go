package desktop

import (
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func testApp(t *testing.T) *core.App {
	t.Helper()
	store, err := core.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return core.NewApp(store, "", t.TempDir())
}
