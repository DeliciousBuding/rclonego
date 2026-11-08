package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowseLocalPagesDirectoriesBeforeFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"beta", "Alpha"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "aardvark.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{}
	first, err := app.Browse(context.Background(), BrowseRequest{Path: directory, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != "local" || len(first.Entries) != 2 || first.Entries[0].Name != "Alpha" || first.Entries[1].Name != "beta" || !first.HasMore {
		t.Fatalf("unexpected first page: %+v", first)
	}
	last, err := app.Browse(context.Background(), BrowseRequest{Path: directory, Offset: first.NextOffset, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Entries) != 1 || last.Entries[0].Size != 9 || last.Entries[0].Path != filepath.Join(directory, "aardvark.txt") || last.HasMore {
		t.Fatalf("unexpected last page: %+v", last)
	}
	dirs, err := app.Browse(context.Background(), BrowseRequest{Path: directory, DirectoriesOnly: true})
	if err != nil || len(dirs.Entries) != 2 {
		t.Fatalf("directory picker includes files: %+v %v", dirs, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.Browse(ctx, BrowseRequest{Path: directory}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestBrowseRejectsAmbiguousPathsAndInvalidPages(t *testing.T) {
	app := &App{}
	for _, value := range []string{"", "relative/folder", ":local:/tmp", "cloud:../outside", "cloud:folder/../outside", "cloud:\nfolder"} {
		if _, err := app.Browse(context.Background(), BrowseRequest{Path: value}); err == nil {
			t.Fatalf("accepted unsafe or ambiguous path %q", value)
		}
	}
	for _, request := range []BrowseRequest{
		{Path: t.TempDir(), Offset: -1},
		{Path: t.TempDir(), Offset: browseScanLimit},
		{Path: t.TempDir(), Limit: browsePageLimit + 1},
	} {
		if _, err := app.Browse(context.Background(), request); err == nil {
			t.Fatalf("accepted invalid pagination %+v", request)
		}
	}
	root, parent, kind, err := browsePath("example:folder/nested/")
	if err != nil || root != "example:folder/nested" || parent != "example:folder" || kind != "remote" {
		t.Fatalf("unexpected remote navigation: %q %q %q %v", root, parent, kind, err)
	}
	root, parent, _, err = browsePath("example:/folder/nested/")
	if err != nil || root != "example:/folder/nested" || parent != "example:/folder" {
		t.Fatalf("absolute remote path was changed to a relative path: %q %q %v", root, parent, err)
	}
	root, parent, _, err = browsePath("example:/")
	if err != nil || root != "example:/" || parent != "" {
		t.Fatalf("incorrect absolute remote root: %q %q %v", root, parent, err)
	}
}

func TestBrowseConfiguredRemoteUsingIsolatedAlias(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.MkdirAll(filepath.Join(source, "folder", "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(directory, "rclone.conf")
	content := "[synthetic]\ntype = alias\nremote = " + filepath.ToSlash(source) + "\n"
	if err := os.WriteFile(config, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{rclone: RcloneClient{Path: engine, ConfigPath: config}}
	root, err := app.Browse(context.Background(), BrowseRequest{Path: "synthetic:", DirectoriesOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Entries) != 1 || root.Entries[0].Path != "synthetic:folder" || root.Parent != "" || !root.Entries[0].IsDir {
		t.Fatalf("unexpected remote root: %+v", root)
	}
	child, err := app.Browse(context.Background(), BrowseRequest{Path: root.Entries[0].Path, DirectoriesOnly: true})
	if err != nil || child.Parent != "synthetic:" || len(child.Entries) != 1 || child.Entries[0].Path != "synthetic:folder/nested" {
		t.Fatalf("unexpected remote child: %+v %v", child, err)
	}
	all, err := app.Browse(context.Background(), BrowseRequest{Path: "synthetic:"})
	if err != nil || len(all.Entries) != 2 || all.Entries[1].Name != "sample.txt" {
		t.Fatalf("unexpected remote file listing: %+v %v", all, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.Browse(ctx, BrowseRequest{Path: "synthetic:"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("remote cancellation lost: %v", err)
	}
	if _, err := app.Browse(context.Background(), BrowseRequest{Path: "missing-remote:"}); err == nil {
		t.Fatal("nonexistent remote was reported as an empty directory")
	}
	if strings.Contains(root.Entries[0].Path, source) {
		t.Fatal("alias leaked its underlying path into navigation")
	}
}
