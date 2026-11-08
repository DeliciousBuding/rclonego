package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestAutomationTransportPersistsConfigurationAndRejectsRuntimeInjection(t *testing.T) {
	server, owner, _ := scopedFixture(t)
	dir := t.TempDir()
	source, target := filepath.Join(dir, "documents"), filepath.Join(dir, "archive")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	request := AutomationRequest{Name: "Documents copy", Kind: "watch", Source: source, Destination: target,
		DebounceSeconds: 10, StableSeconds: 10, RescanSeconds: 3600}
	recorder := apiRequest(t, server, owner, "POST", "/api/v1/automations", request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create automation status %d: %s", recorder.Code, recorder.Body.String())
	}
	var saved core.Automation
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Enabled || saved.Source != source || saved.Kind != "watch" {
		t.Fatal("automation configuration was not preserved")
	}
	injected := map[string]any{"name": "Injected", "kind": "watch", "source": source, "destination": target,
		"enabled": true, "pending_key": "foreign-run", "status": "running"}
	if recorder := apiRequest(t, server, owner, "POST", "/api/v1/automations", injected); recorder.Code != 400 {
		t.Fatal("runtime automation fields were accepted by the transport")
	}
	if len(server.app.ListAutomations()) != 1 {
		t.Fatal("runtime injection modified automation state")
	}
	request.ID = "different-id"
	if recorder := apiRequest(t, server, owner, "PUT", "/api/v1/automations/"+saved.ID, request); recorder.Code != 400 {
		t.Fatal("conflicting automation ID was accepted")
	}
	if recorder := apiRequest(t, server, owner, "DELETE", "/api/v1/automations/"+saved.ID, nil); recorder.Code != 200 {
		t.Fatal("automation could not be removed")
	}
}
