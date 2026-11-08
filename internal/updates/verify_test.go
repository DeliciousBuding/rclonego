package updates

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSignedReleaseArtifactsRejectTampering(t *testing.T) {
	directory := t.TempDir()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("synthetic signed release archive")
	archive := filepath.Join(directory, "example-1.0.0-windows-amd64.tar.gz")
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	release := map[string]any{
		"version":   "1.0.0",
		"url":       "https://github.com/example/example/releases/download/v1.0.0/" + filepath.Base(archive),
		"size":      len(data),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(private, digest[:])),
	}
	writeManifest := func() {
		raw, err := json.Marshal(release)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "update-windows-amd64.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest()
	key := base64.StdEncoding.EncodeToString(public)
	if count, err := VerifyDirectory(directory, key, "1.0.0", true); err != nil || count != 1 {
		t.Fatalf("valid official-format artifact: count=%d err=%v", count, err)
	}
	if err := os.WriteFile(archive, []byte("tampered  signed release archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(directory, key, "1.0.0", true); err == nil {
		t.Fatal("modified archive accepted")
	}
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(directory, key, "2.0.0", true); err == nil {
		t.Fatal("manifest for another release accepted")
	}
	release["url"] = "file:///private/archive.tar.gz"
	writeManifest()
	if _, err := VerifyDirectory(directory, key, "1.0.0", true); err == nil {
		t.Fatal("non-HTTPS artifact accepted")
	}
}

func TestRequiredSignedReleaseRejectsMissingManifest(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(t.TempDir(), base64.StdEncoding.EncodeToString(public), "", true); err == nil {
		t.Fatal("required signing accepted an unsigned release")
	}
}
