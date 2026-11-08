package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncPreviewCannotDeleteAndConfirmationBindsParameters(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	dir := t.TempDir()
	source, target, data := filepath.Join(dir, "source"), filepath.Join(dir, "target"), filepath.Join(dir, "state")
	for _, path := range []string{source, target} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(source, "sample.txt"), []byte("synthetic"), 0o600)
	os.WriteFile(filepath.Join(target, "extra.txt"), []byte("preserve until confirmed"), 0o600)
	os.WriteFile(filepath.Join(source, "update.txt"), []byte("new synthetic version"), 0o600)
	os.WriteFile(filepath.Join(target, "update.txt"), []byte("old"), 0o600)
	for _, directory := range []string{source, target} {
		unchanged := filepath.Join(directory, "unchanged.txt")
		os.WriteFile(unchanged, []byte("same synthetic content"), 0o600)
		stamp := time.Unix(1700000000, 0)
		os.Chtimes(unchanged, stamp, stamp)
	}
	config := filepath.Join(dir, "rclone.conf")
	os.WriteFile(config, nil, 0o600)
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	request := JobCreateRequest{Name: "Synthetic mirror", Operation: "sync", Source: source, Destination: target, AllowDelete: true}
	if _, err := app.AddJob(request); err == nil {
		t.Fatal("mirror accepted without preview")
	}
	preview, err := app.PreviewJob(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Operation != "sync" || preview.Summary.Added != 1 || preview.Summary.Changed != 1 ||
		preview.Summary.Deleted != 1 || preview.Summary.Unchanged != 1 || preview.ReportIncomplete {
		t.Fatalf("incorrect structured comparison: %+v", preview)
	}
	actions := map[string]string{}
	for _, entry := range preview.Entries {
		actions[entry.Path] = entry.Action
	}
	if actions["sample.txt"] != "added" || actions["update.txt"] != "changed" || actions["extra.txt"] != "deleted" {
		t.Fatalf("incorrect preview actions: %+v", actions)
	}
	if _, err := os.Stat(filepath.Join(target, "extra.txt")); err != nil {
		t.Fatal("preview deleted destination file")
	}
	if _, err := os.Stat(filepath.Join(target, "sample.txt")); !os.IsNotExist(err) {
		t.Fatal("preview copied file")
	}
	request.PreviewID = preview.ID
	modified := request
	modified.Destination = filepath.Join(dir, "another-target")
	if _, err := app.AddJob(modified); err == nil {
		t.Fatal("accepted mismatched preview")
	}
	job, err := app.AddJob(request)
	if err != nil {
		t.Fatal(err)
	}
	app.Start()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, current := range app.Snapshot().Jobs {
			if current.ID != job.ID {
				continue
			}
			if current.Status == "failed" {
				t.Fatal(current.Error)
			}
			if current.Status == "completed" {
				if _, err := os.Stat(filepath.Join(target, "extra.txt")); !os.IsNotExist(err) {
					t.Fatal("confirmed mirror did not delete extra file")
				}
				if _, err := os.Stat(filepath.Join(target, "sample.txt")); err != nil {
					t.Fatal("confirmed mirror did not copy file")
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("mirror did not complete")
}

func TestPreviewFileSelectionMatchesExecutionAndCopyDoesNotDelete(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	directory := t.TempDir()
	source, target, data := filepath.Join(directory, "source"), filepath.Join(directory, "target"), filepath.Join(directory, "data")
	for _, folder := range []string{source, target} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"+ chosen file.txt", "outside.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "preserve.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(directory, "rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	preview, err := app.PreviewJob(context.Background(), JobCreateRequest{
		Name: "Synthetic selected copy", Operation: "copy", Source: source, Destination: target,
		Files: []string{"+ chosen file.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Summary.Added != 1 || preview.Summary.Deleted != 0 || preview.ReportIncomplete ||
		len(preview.Entries) != 1 || preview.Entries[0].Path != "+ chosen file.txt" || preview.Entries[0].Action != "added" {
		t.Fatalf("preview lost file scope or invented a deletion: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(target, "+ chosen file.txt")); !os.IsNotExist(err) {
		t.Fatal("preview mutated the target")
	}
	entries, err := os.ReadDir(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".preview-") {
			t.Fatal("preview left a temporary report behind")
		}
	}
}

func TestPreviewComparisonOutputIsBoundedAndIndependentFromLogs(t *testing.T) {
	report := &previewReportOutput{operation: "copy"}
	for i := 0; i < 900; i++ {
		report.Write([]byte("+ path with spaces.txt\n"))
	}
	report.Write([]byte("- existing-only.txt\n"))
	report.Write([]byte("= unchanged.txt\n"))
	report.finish()
	if len(report.entries) != 500 || !report.truncated || report.summary.Added != 900 ||
		report.summary.Deleted != 0 || report.summary.DestinationOnly != 1 || report.summary.Unchanged != 1 || report.incomplete {
		t.Fatalf("unbounded or incorrect report: entries=%d summary=%+v", len(report.entries), report.summary)
	}
	report.Write([]byte(strings.Repeat("x", 40<<10) + "\n"))
	report.Write([]byte(`{"msg":"Deleted a file"}` + "\n"))
	if !report.incomplete || len(report.pending) != 0 {
		t.Fatal("accepted logs as authoritative changes or retained an overlong line")
	}
	log := &boundedPreviewOutput{limit: 8}
	log.Write([]byte("123456"))
	log.Write([]byte("7890"))
	if string(log.data) != "34567890" || !log.truncated {
		t.Fatalf("output tail is not bounded: %q", log.data)
	}
	log.Write([]byte("abcdefghijk"))
	if string(log.data) != "defghijk" {
		t.Fatalf("large writes exceeded the bound: %q", log.data)
	}
}
