package core

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAccountConfigurationCommitsAfterVerificationAndCancelPreservesConfig(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "state")
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	config := filepath.Join(dir, "rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	account, err := app.BeginAccount(context.Background(), AccountRequest{Name: "synthetic", Type: "alias", Parameters: map[string]string{"remote": dir}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		account, err = app.Account(account.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch account.Status {
		case "needs_input":
			answer := account.Question.DefaultStr
			if account.Question.Name == "remote" {
				answer = dir
			}
			if err := app.AnswerAccount(account.ID, answer); err != nil {
				t.Fatal(err)
			}
		case "failed", "cancelled":
			t.Fatalf("configuration failed: %+v", account)
		case "completed":
			if !account.Verified {
				t.Fatal("completed without read verification")
			}
			goto verified
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("configuration timed out")
verified:
	baseline, err := os.ReadFile(config)
	if err != nil || !bytes.Contains(baseline, []byte("[synthetic]")) {
		t.Fatalf("config not committed: %v", err)
	}
	account, err = app.BeginAccount(context.Background(), AccountRequest{Name: "cancel-example", Type: "alias"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.BeginAccount(context.Background(), AccountRequest{Name: "writer-conflict", Type: "alias"}); err == nil {
		t.Fatal("accepted concurrent config writer")
	}
	if err := app.CancelAccount(account.ID); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(config)
	if err != nil || !bytes.Equal(actual, baseline) {
		t.Fatal("cancel changed original config")
	}
}

func TestSecretQuestionDoesNotExposeDefault(t *testing.T) {
	question := ConfigQuestion{Name: "password", IsPassword: true, DefaultStr: "synthetic secret"}
	sanitizeQuestion(&question)
	if question.DefaultStr != "" {
		t.Fatal("password default exposed")
	}
}
