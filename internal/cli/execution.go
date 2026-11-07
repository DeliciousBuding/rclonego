package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func executionRequest(args []string) (method, path string, body any, key string, err error) {
	method = http.MethodGet
	switch args[0] {
	case "doctor":
		if len(args) != 1 {
			err = errors.New("use doctor without positional arguments")
			return
		}
		path = "/api/v1/doctor"
	case "workflows":
		path = "/api/v1/workflow-definitions"
		if len(args) == 1 || len(args) == 2 && args[1] == "list" {
			return
		}
		if len(args) == 3 && args[1] == "delete" {
			method, path = http.MethodDelete, path+"/"+url.PathEscape(args[2])
			return
		}
		if len(args) == 4 && args[1] == "save" && args[2] == "--file" {
			raw, e := os.ReadFile(args[3])
			if e != nil {
				err = e
				return
			}
			var definition core.WorkflowDefinition
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&definition); err != nil {
				return
			}
			if e := decoder.Decode(&struct{}{}); e != io.EOF {
				err = errors.New("workflow file must contain one JSON value")
				return
			}
			method, body = http.MethodPost, definition
			return
		}
		if len(args) == 3 && args[1] == "run" {
			method, path, body = http.MethodPost, "/api/v1/runs", map[string]string{"workflow_id": args[2]}
			return
		}
		if len(args) == 5 && args[1] == "run" && args[3] == "--idempotency-key" {
			method, path, body, key = http.MethodPost, "/api/v1/runs", map[string]string{"workflow_id": args[2]}, args[4]
			return
		}
		err = errors.New("use workflows list, save --file <json>, or run <id>")
	case "runs":
		path = "/api/v1/runs"
		if len(args) == 1 || len(args) == 2 && args[1] == "list" {
			return
		}
		if len(args) > 2 && args[1] == "list" {
			set := flag.NewFlagSet("runs list", flag.ContinueOnError)
			set.SetOutput(io.Discard)
			offset := set.Int("offset", 0, "")
			limit := set.Int("limit", 100, "")
			err = set.Parse(args[2:])
			if err == nil && set.NArg() != 0 {
				err = errors.New("unexpected run list arguments")
			}
			path += fmt.Sprintf("?offset=%d&limit=%d", *offset, *limit)
			return
		}
		if len(args) == 3 && (args[1] == "report" || args[1] == "wait" || args[1] == "watch") {
			path += "/" + url.PathEscape(args[2]) + "/report"
			return
		}
		if len(args) == 3 && (args[1] == "retry" || args[1] == "pause" || args[1] == "resume" || args[1] == "cancel") {
			method, path, body = http.MethodPost, path+"/"+url.PathEscape(args[2])+"/"+args[1], map[string]string{}
			return
		}
		err = errors.New("use runs list, report, wait, or watch <id>")
	}
	return
}

func printExecution(out io.Writer, args []string, raw []byte) error {
	if args[0] == "doctor" {
		var items []core.Diagnostic
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		for _, item := range items {
			fmt.Fprintf(out, "%s: %s — %s\n", item.ID, item.Status, item.Message)
			if item.NextAction != "" {
				fmt.Fprintf(out, "  %s\n", item.NextAction)
			}
		}
		return nil
	}
	if args[0] == "runs" && len(args) == 3 && (args[1] == "report" || args[1] == "wait" || args[1] == "watch") {
		var report core.RunReport
		if err := json.Unmarshal(raw, &report); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s · %s\n校验: %s\n执行尝试: %d\n传输流量: %s\n", report.Run.Name, report.Status, report.Verification, len(report.Attempts), formatBytes(report.WireBytes))
		for _, step := range report.Steps {
			fmt.Fprintf(out, "  %d/%d %s · %s · 尝试 %d\n", step.WorkflowStep, step.WorkflowSize, step.Name, statusLabel(step.Status), step.Attempt)
		}
		return nil
	}
	var records []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &records); err == nil {
		for _, record := range records {
			fmt.Fprintf(out, "%s  %s\n", record.ID, record.Name)
		}
		if len(records) == 0 {
			fmt.Fprintln(out, "暂无记录")
		}
		return nil
	}
	var record struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s  %s\n", record.ID, record.Name)
	return nil
}

func waitRun(ctx context.Context, options cliOptions, args []string, out, stderr io.Writer) int {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	path := "/api/v1/runs/" + url.PathEscape(args[2]) + "/report"
	previous := ""
	for {
		raw, err := callAPI(ctx, options.api, http.MethodGet, path, nil, "")
		if err != nil {
			return cliError(stderr, options.json, err)
		}
		var report core.RunReport
		if err = json.Unmarshal(raw, &report); err != nil {
			return cliError(stderr, options.json, err)
		}
		terminal := report.Status == "completed" || report.Status == "failed" || report.Status == "interrupted" || report.Status == "cancelled"
		if report.Status == "paused" {
			if options.json {
				fmt.Fprintln(out, string(raw))
			} else {
				_ = printExecution(out, args, raw)
			}
			return 4
		}
		if terminal || args[1] == "watch" && report.Status != previous {
			if options.json {
				fmt.Fprintln(out, string(raw))
			} else {
				_ = printExecution(out, args, raw)
			}
			previous = report.Status
		}
		if terminal {
			if report.Status == "completed" {
				return 0
			}
			return 1
		}
		select {
		case <-ctx.Done():
			_ = cliError(stderr, options.json, ctx.Err())
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return 3
			}
			return 130
		case <-ticker.C:
		}
	}
}
