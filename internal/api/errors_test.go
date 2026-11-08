package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestTypedFilesystemAndContextErrorsKeepRecoverySemantics(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{context.DeadlineExceeded, 504, "timeout"},
		{context.Canceled, 408, "cancelled"},
		{os.ErrPermission, 403, "filesystem_permission_denied"},
		{os.ErrNotExist, 404, "path_not_found"},
		{errors.New("invalid choice"), 400, "invalid_request"},
	} {
		recorder := httptest.NewRecorder()
		writeFailure(recorder, 400, test.err)
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("wrong typed failure for %s: %d %s", test.code, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPersistenceFailureIsReportedAsServiceBlocker(t *testing.T) {
	dir := t.TempDir()
	store, err := core.OpenStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	app := core.NewApp(store, "", dir)
	owner := strings.Repeat("o", 64)
	server, err := New(app, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := apiRequest(t, server, owner, "POST", "/api/v1/jobs",
		core.JobCreateRequest{Name: "Synthetic", Operation: "copy", Source: "source", Destination: "destination"})
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":"storage_unavailable"`) {
		t.Fatalf("persistence failure was misclassified: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(app.Snapshot().Jobs) != 0 {
		t.Fatal("failed persisted submission modified the authoritative queue")
	}
}
