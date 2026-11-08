package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func cacheServerPort(t *testing.T, server *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, text, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(text)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestMountCacheReadsOfficialCountersAndLimitsUploadDetails(t *testing.T) {
	const token = "synthetic-rc-authentication"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		user, password, ok := r.BasicAuth()
		if !ok || user != "rclonego" || password != token || r.Method != http.MethodPost {
			t.Error("VFS status request did not authenticate")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request["fs"] != "fixture:archive" {
			t.Error("VFS status selected a different filesystem")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/vfs/stats":
			fmt.Fprint(w, `{"fs":"fixture:archive","inUse":3,"metadataCache":{"dirs":4,"files":9},"diskCache":{"bytesUsed":1048576,"files":250,"erroredFiles":2,"outOfSpace":true,"uploadsInProgress":2,"uploadsQueued":205,"path":"/synthetic/cache","pathMeta":"/synthetic/meta"},"opt":{"CacheMode":3}}`)
		case "/vfs/queue":
			items := make([]VFSUpload, 205)
			for i := range items {
				items[i] = VFSUpload{Name: fmt.Sprintf("document-%03d.bin", i), ID: uint64(i + 1), Size: 12, ExpirySeconds: -2.5, Tries: 2, DelaySeconds: 5, Uploading: i < 2}
			}
			json.NewEncoder(w).Encode(map[string]any{"queue": items})
		default:
			t.Error("unexpected VFS command")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	got := sampleMountCache(context.Background(), RcloneClient{RCToken: token}, cacheServerPort(t, server), MountConfig{ID: "fixture", Remote: "fixture:archive"})
	if !got.Available || got.Status != "ready" || got.DiskCache == nil || got.DiskCache.BytesUsed != 1048576 || !got.DiskCache.OutOfSpace || got.DiskCache.ErroredFiles != 2 {
		t.Fatalf("official disk cache counters lost: %+v", got)
	}
	if got.ReferenceCount != 3 || got.MetadataCache.Directories != 4 || got.MetadataCache.Files != 9 {
		t.Fatalf("VFS and metadata counters changed meaning: %+v", got)
	}
	if len(got.Queue) != mountCacheQueueLimit || !got.QueueTruncated || got.Queue[0].ExpirySeconds != -2.5 || !got.Queue[0].Uploading || got.Queue[0].Tries != 2 || calls.Load() != 2 {
		t.Fatalf("queue bounds or eligibility fields lost: count=%d, %+v", len(got.Queue), got)
	}
}

func TestMountCacheRejectsUnownedOrChangedProcessIdentity(t *testing.T) {
	app := testApp(t)
	cfg := MountConfig{ID: "fixture", Name: "Synthetic mount", Remote: "fixture:", Drive: "Q:", CacheDir: t.TempDir()}
	if err := app.UpdateState(func(state *State) error { state.Mounts = []MountConfig{cfg}; return nil }); err != nil {
		t.Fatal(err)
	}
	view, err := app.MountCache(context.Background(), cfg.ID)
	if err != nil || view.Available || view.Status != "not_owned" || len(view.Queue) != 0 {
		t.Fatalf("unowned mount was sampled: %+v / %v", view, err)
	}
	if err := app.UpdateState(func(state *State) error {
		state.OwnedMounts = map[string]int{cfg.ID: os.Getpid()}
		state.MountIdentities = map[string]string{cfg.ID: "synthetic-stale-start-identity"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err = app.MountCache(context.Background(), cfg.ID)
	if err != nil || view.Available || view.Status != "identity_changed" {
		t.Fatalf("changed PID identity was accepted: %+v / %v", view, err)
	}
	if _, err := app.MountCache(context.Background(), "missing"); err == nil {
		t.Fatal("unknown mount was accepted")
	}
}

func TestMountCacheRejectsRedirectsOversizedAndInvalidResponses(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		fmt.Fprint(w, `{}`)
	}))
	defer target.Close()
	for _, scenario := range []string{"redirect", "oversized", "invalid", "missing", "negative"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
				case "oversized":
					fmt.Fprint(w, `{"metadataCache":{"dirs":0,"files":0},"inUse":1,"ignored":"`+strings.Repeat("x", mountCacheResponseLimit)+`"}`)
				case "invalid":
					fmt.Fprint(w, `not-json`)
				case "missing":
					fmt.Fprint(w, `{}`)
				case "negative":
					fmt.Fprint(w, `{"metadataCache":{"dirs":0,"files":0},"inUse":1,"diskCache":{"bytesUsed":-1}}`)
				}
			}))
			defer server.Close()
			view := sampleMountCache(context.Background(), RcloneClient{RCToken: "fixture-token"}, cacheServerPort(t, server), MountConfig{ID: "fixture", Remote: "fixture:"})
			if view.Available || view.Error == "" || len(view.Queue) != 0 {
				t.Fatalf("%s response was trusted: %+v", scenario, view)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("VFS redirect was followed")
	}
}

func TestMountCacheKeepsDiskStatusWhenQueueDetailsAreUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/vfs/stats" {
			fmt.Fprint(w, `{"inUse":1,"metadataCache":{"dirs":1,"files":1},"diskCache":{"bytesUsed":8,"files":1,"uploadsQueued":300}}`)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	view := sampleMountCache(context.Background(), RcloneClient{}, cacheServerPort(t, server), MountConfig{ID: "fixture", Remote: "fixture:"})
	if !view.Available || view.DiskCache == nil || view.DiskCache.UploadsQueued != 300 || view.QueueError == "" || !view.QueueTruncated {
		t.Fatalf("unavailable queue hid authoritative disk counters: %+v", view)
	}
}

func TestOfflineCopyKeepsSharedQueueSemanticsAndRejectsCacheDestinations(t *testing.T) {
	app := testApp(t)
	destination := t.TempDir()
	cache := t.TempDir()
	if err := app.UpdateState(func(state *State) error {
		state.Mounts = []MountConfig{{ID: "fixture", CacheDir: cache}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, request := range []OfflineCopyRequest{
		{Source: "fixture:archive", Destination: filepath.Join(cache, "offline")},
		{Source: destination, Destination: destination},
		{Source: ":local:archive", Destination: destination},
		{Source: "fixture:../archive", Destination: destination},
		{Source: "fixture:archive", Destination: "relative"},
		{Source: "fixture:", Destination: destination, File: true},
	} {
		if _, err := app.AddOfflineCopy(request); err == nil {
			t.Fatalf("invalid offline copy accepted: %+v", request)
		}
	}
	request := OfflineCopyRequest{Source: "fixture:archive/vault.bin", Destination: destination, File: true, IdempotencyKey: "synthetic-offline"}
	job, err := app.AddOfflineCopy(request)
	if err != nil {
		t.Fatal(err)
	}
	if job.Operation != "copyto" || job.Destination != filepath.Join(destination, "vault.bin") || job.AllowDelete || job.Status != "pending" {
		t.Fatalf("offline copy bypassed shared copy semantics: %+v", job)
	}
	repeated, err := app.AddOfflineCopy(request)
	if err != nil || repeated.ID != job.ID || len(app.Snapshot().Jobs) != 1 {
		t.Fatalf("offline copy duplicated an idempotent request: %+v / %v", repeated, err)
	}
	request.Source = "fixture:another.bin"
	if _, err := app.AddOfflineCopy(request); err == nil {
		t.Fatal("different offline copy reused an idempotency key")
	}
	if len(app.Snapshot().Jobs) != 1 {
		t.Fatal("invalid requests created extra jobs")
	}
	link := filepath.Join(t.TempDir(), "cache-link")
	if err := os.Symlink(cache, link); err == nil {
		if _, err := app.AddOfflineCopy(OfflineCopyRequest{Source: "fixture:archive", Destination: filepath.Join(link, "offline")}); err == nil {
			t.Fatal("symlink redirected offline files into the VFS cache")
		}
	}
	dangling := filepath.Join(t.TempDir(), "missing-cache-link")
	if err := os.Symlink(filepath.Join(cache, "not-created"), dangling); err == nil {
		if _, err := app.AddOfflineCopy(OfflineCopyRequest{Source: "fixture:archive", Destination: filepath.Join(dangling, "offline")}); err == nil {
			t.Fatal("unresolved symlink was accepted as an offline destination")
		}
	}
}

func TestOfflineCopyIsDurableAndPreservesDestinationExtras(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("integration test requires a verified rclone executable")
	}
	root := t.TempDir()
	source, destination, data := filepath.Join(root, "source"), filepath.Join(root, "offline"), filepath.Join(root, "state")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Repeat([]byte("synthetic offline vault\n"), 512)
	if err := os.WriteFile(filepath.Join(source, "vault.bin"), expected, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "keep.txt"), []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "rclone.conf")
	if err := os.WriteFile(config, []byte("[fixture]\ntype = alias\nremote = "+filepath.ToSlash(source)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	job, err := app.AddOfflineCopy(OfflineCopyRequest{Source: "fixture:", Destination: destination, IdempotencyKey: "offline-restart"})
	if err != nil {
		app.Close()
		store.Close()
		t.Fatal(err)
	}
	app.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app = NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	if persisted, err := app.store.JobByKey("offline-restart"); err != nil || persisted.ID != job.ID {
		t.Fatalf("offline task did not survive restart: %+v / %v", persisted, err)
	}
	app.Start()
	deadline := time.Now().Add(15 * time.Second)
	completed := false
	for time.Now().Before(deadline) {
		for _, current := range app.Snapshot().Jobs {
			if current.ID != job.ID {
				continue
			}
			if current.Status == "failed" {
				t.Fatalf("offline copy failed: %s", current.Error)
			}
			completed = current.Status == "completed"
		}
		if completed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !completed {
		t.Fatal("offline copy did not complete")
	}
	actual, err := os.ReadFile(filepath.Join(destination, "vault.bin"))
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatalf("offline content differs: %v", err)
	}
	if existing, err := os.ReadFile(filepath.Join(destination, "keep.txt")); err != nil || string(existing) != "preserve" {
		t.Fatal("offline copy removed a destination-only file")
	}
}
