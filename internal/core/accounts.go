package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ConfigExample struct {
	Value string `json:"Value"`
	Help  string `json:"Help"`
}

type ConfigQuestion struct {
	Name       string          `json:"Name"`
	Help       string          `json:"Help"`
	DefaultStr string          `json:"DefaultStr"`
	Type       string          `json:"Type"`
	Required   bool            `json:"Required"`
	IsPassword bool            `json:"IsPassword"`
	Sensitive  bool            `json:"Sensitive"`
	Exclusive  bool            `json:"Exclusive"`
	Examples   []ConfigExample `json:"Examples,omitempty"`
}

type Provider struct {
	Name        string           `json:"Name"`
	Description string           `json:"Description"`
	Options     []ConfigQuestion `json:"Options"`
}

type AccountRequest struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Reconnect  bool              `json:"reconnect,omitempty"`
}

type AccountView struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Status    string          `json:"status"`
	Question  *ConfigQuestion `json:"question,omitempty"`
	Message   string          `json:"message,omitempty"`
	Verified  bool            `json:"verified"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type accountSession struct {
	mu            sync.Mutex
	view          AccountView
	client        RcloneClient
	port          int
	draft         string
	baseline      []byte
	state         string
	defaultAnswer string
	cmd           *exec.Cmd
	cancel        context.CancelFunc
	finished      bool
}

// Config RC runs only on a private loopback port with independent auth.
// Request bodies and rclone stderr are never copied into application logs.
func (r RcloneClient) configCall(ctx context.Context, port int, endpoint string, input any, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/%s", port, endpoint), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("rclonego", r.RCToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("configuration engine is not reachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("configuration engine rejected %s (HTTP %d); check the current answer and provider requirements", endpoint, resp.StatusCode)
	}
	if output == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(output)
}

func (a *App) Providers(ctx context.Context) ([]Provider, error) {
	client := a.EngineClient()
	raw, err := client.command(ctx, "rc", "--loopback", "config/providers")
	if err != nil {
		return nil, errors.New("provider discovery requires an installed rclone engine")
	}
	var result struct {
		Providers []Provider `json:"providers"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	for i := range result.Providers {
		for j := range result.Providers[i].Options {
			sanitizeQuestion(&result.Providers[i].Options[j])
		}
	}
	return result.Providers, nil
}

func sanitizeQuestion(q *ConfigQuestion) {
	if q.IsPassword || q.Sensitive {
		q.DefaultStr = ""
	}
}

func (a *App) BeginAccount(ctx context.Context, request AccountRequest) (AccountView, error) {
	if !remotePattern.MatchString(EnsureColonRemote(request.Name)) {
		return AccountView{}, errors.New("account name must use letters, digits, dots, hyphens or underscores")
	}
	if !a.configMu.TryLock() {
		return AccountView{}, errors.New("another account configuration is in progress; finish or cancel it first")
	}
	release := true
	defer func() {
		if release {
			a.configMu.Unlock()
		}
	}()
	providers, err := a.Providers(ctx)
	if err != nil {
		return AccountView{}, err
	}
	client := a.EngineClient()
	remotes, err := client.Remotes(ctx)
	if err != nil {
		return AccountView{}, err
	}
	exists := false
	for _, remote := range remotes {
		if strings.TrimSuffix(remote.Name, ":") == request.Name {
			exists = true
			if request.Reconnect {
				request.Type = remote.Type
			}
		}
	}
	if exists != request.Reconnect {
		return AccountView{}, errors.New("create requires a new name; reconnect requires an existing account")
	}
	var provider *Provider
	for i := range providers {
		if providers[i].Name == request.Type {
			provider = &providers[i]
			break
		}
	}
	if provider == nil {
		return AccountView{}, errors.New("unsupported provider")
	}
	allowed := map[string]bool{}
	for _, option := range provider.Options {
		allowed[option.Name] = true
	}
	for name := range request.Parameters {
		if !allowed[name] {
			return AccountView{}, errors.New("unknown provider option")
		}
	}
	if !request.Reconnect && request.Type == "drive" && request.Parameters["client_id"] == "" {
		return AccountView{}, errors.New("Google Drive setup requires your own OAuth client_id; see docs/accounts.md")
	}
	baseline, err := os.ReadFile(client.ConfigPath)
	if err != nil {
		return AccountView{}, errors.New("selected configuration cannot be read")
	}
	id, err := RandomID()
	if err != nil {
		return AccountView{}, err
	}
	draftFile, err := os.CreateTemp(a.dataDir, ".account-*.conf")
	if err != nil {
		return AccountView{}, err
	}
	draft := draftFile.Name()
	if err = draftFile.Chmod(0o600); err == nil {
		_, err = draftFile.Write(baseline)
	}
	closeErr := draftFile.Close()
	if err != nil || closeErr != nil {
		os.Remove(draft)
		return AccountView{}, errors.New("cannot create private configuration draft")
	}
	port, err := freePort()
	if err != nil {
		os.Remove(draft)
		return AccountView{}, err
	}
	sessionCtx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	draftClient := client
	draftClient.ConfigPath = draft
	cmd := exec.CommandContext(sessionCtx, client.Path, "rcd", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", port), "--rc-job-expire-duration", "1m")
	cmd.Env = draftClient.environment()
	hideProcess(cmd)
	if err := cmd.Start(); err != nil {
		cancel()
		os.Remove(draft)
		return AccountView{}, err
	}
	session := &accountSession{view: AccountView{ID: id, Name: request.Name, Provider: request.Type, Status: "configuring", UpdatedAt: time.Now().UTC()},
		client: draftClient, port: port, draft: draft, baseline: baseline, cmd: cmd, cancel: cancel}
	a.accountsMu.Lock()
	// Only the latest session is kept; credentials are kept solely in the draft.
	a.accounts = map[string]*accountSession{id: session}
	a.accountsMu.Unlock()
	release = false
	initialView := session.view
	go func() {
		<-sessionCtx.Done()
		session.mu.Lock()
		defer session.mu.Unlock()
		if !session.finished {
			session.view.Status, session.view.Message = "cancelled", "configuration session expired or service stopped"
			a.endAccountLocked(session)
		}
	}()
	go func() {
		readyCtx, readyCancel := context.WithTimeout(sessionCtx, 8*time.Second)
		defer readyCancel()
		for {
			if draftClient.configCall(readyCtx, port, "core/version", map[string]any{}, nil) == nil {
				break
			}
			select {
			case <-readyCtx.Done():
				session.mu.Lock()
				if !session.finished {
					session.view.Status, session.view.Message = "failed", "configuration engine did not start"
					a.endAccountLocked(session)
				}
				session.mu.Unlock()
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		parameters := request.Parameters
		if parameters == nil {
			parameters = map[string]string{}
		}
		opt := map[string]any{"nonInteractive": true, "all": !request.Reconnect}
		if request.Reconnect {
			parameters["config_refresh_token"] = "true"
		}
		endpoint := "config/create"
		input := map[string]any{"name": request.Name, "type": request.Type, "parameters": parameters, "opt": opt}
		if request.Reconnect {
			endpoint = "config/update"
		}
		a.accountStep(sessionCtx, session, endpoint, input)
	}()
	return initialView, nil
}

func (a *App) accountStep(ctx context.Context, session *accountSession, endpoint string, input any) {
	var out struct {
		State  string          `json:"State"`
		Option *ConfigQuestion `json:"Option"`
		Error  string          `json:"Error"`
	}
	err := session.client.configCall(ctx, session.port, endpoint, input, &out)
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finished {
		return
	}
	session.view.UpdatedAt = time.Now().UTC()
	if err != nil {
		session.view.Status, session.view.Message = "failed", err.Error()
		a.endAccountLocked(session)
		return
	}
	session.state = out.State
	if out.Option != nil {
		session.defaultAnswer = out.Option.DefaultStr
		sanitizeQuestion(out.Option)
		session.view.Question, session.view.Status = out.Option, "needs_input"
		session.view.Message = ""
		if out.Error != "" {
			session.view.Message = "The previous answer was rejected. Review the provider question."
		}
		return
	}
	if out.State != "" {
		session.view.Status, session.view.Message = "failed", "provider returned a state without a question"
		a.endAccountLocked(session)
		return
	}
	// Verify basic read access in the draft before replacing the selected config.
	session.view.Status = "verifying"
	session.mu.Unlock()
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	_, verifyErr := session.client.command(verifyCtx, "lsjson", EnsureColonRemote(session.view.Name), "--max-depth", "1", "--dirs-only")
	cancel()
	session.mu.Lock()
	if session.finished {
		return
	}
	if verifyErr != nil {
		session.view.Status, session.view.Message = "failed", "configuration completed but read access could not be verified; original configuration preserved"
		a.endAccountLocked(session)
		return
	}
	config := a.EngineClient().ConfigPath
	current, readErr := os.ReadFile(config)
	if readErr != nil || !bytes.Equal(current, session.baseline) {
		session.view.Status, session.view.Message = "failed", "configuration changed outside this session; original preserved, start again"
		a.endAccountLocked(session)
		return
	}
	if filepath.Dir(config) != filepath.Dir(session.draft) {
		raw, err := os.ReadFile(session.draft)
		if err == nil {
			var temp *os.File
			temp, err = os.CreateTemp(filepath.Dir(config), ".rclonego-config-*")
			if err == nil {
				_ = temp.Chmod(0o600)
				_, err = temp.Write(raw)
				if err == nil {
					err = temp.Sync()
				}
				closeErr := temp.Close()
				if err == nil {
					err = closeErr
				}
				if err == nil {
					err = os.Rename(temp.Name(), config)
				}
				_ = os.Remove(temp.Name())
			}
		}
		readErr = err
	} else {
		readErr = os.Rename(session.draft, config)
	}
	if readErr != nil {
		session.view.Status, session.view.Message = "failed", "could not commit configuration; original preserved"
	} else {
		session.view.Status, session.view.Message, session.view.Verified = "completed", "read access verified", true
	}
	a.endAccountLocked(session)
	a.RefreshRuntime()
}

func (a *App) endAccountLocked(session *accountSession) {
	if session.finished {
		return
	}
	session.finished = true
	session.view.Question = nil
	session.cancel()
	_ = session.cmd.Wait()
	_ = os.Remove(session.draft)
	session.baseline, session.state = nil, ""
	session.defaultAnswer = ""
	a.configMu.Unlock()
}

func (a *App) Account(id string) (AccountView, error) {
	a.accountsMu.Lock()
	session := a.accounts[id]
	a.accountsMu.Unlock()
	if session == nil {
		return AccountView{}, errors.New("account session not found")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	view := session.view
	if view.Question != nil {
		copy := *view.Question
		copy.Examples = append([]ConfigExample(nil), copy.Examples...)
		view.Question = &copy
	}
	return view, nil
}

func (a *App) AnswerAccount(id, answer string) error {
	a.accountsMu.Lock()
	session := a.accounts[id]
	a.accountsMu.Unlock()
	if session == nil {
		return errors.New("account session not found")
	}
	session.mu.Lock()
	if session.finished || session.view.Status != "needs_input" {
		session.mu.Unlock()
		return errors.New("account session is not waiting for an answer")
	}
	state := session.state
	name := session.view.Name
	if answer == "" {
		answer = session.defaultAnswer
	}
	session.view.Status, session.view.Question = "configuring", nil
	session.mu.Unlock()
	go a.accountStep(context.Background(), session, "config/update", map[string]any{
		"name": name, "parameters": map[string]string{},
		"opt": map[string]any{"nonInteractive": true, "continue": true, "all": true, "state": state, "result": answer}})
	return nil
}

func (a *App) CancelAccount(id string) error {
	a.accountsMu.Lock()
	session := a.accounts[id]
	a.accountsMu.Unlock()
	if session == nil {
		return errors.New("account session not found")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finished {
		return nil
	}
	session.view.Status, session.view.Message = "cancelled", "original configuration preserved"
	a.endAccountLocked(session)
	return nil
}
