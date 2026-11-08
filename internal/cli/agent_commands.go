package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func agentRequest(args []string) (method, path string, body any, key string, err error) {
	method = http.MethodGet
	switch args[0] {
	case "mounts":
		if len(args) != 3 || args[1] != "cache" {
			err = errors.New("use mounts cache <id>")
			return
		}
		path = "/api/v1/mounts/" + url.PathEscape(args[2]) + "/cache"
	case "offline":
		if len(args) != 3 || args[1] != "--file" {
			err = errors.New("use offline --file <copy.json>")
			return
		}
		var value core.OfflineCopyRequest
		err = readCommandFile(args[2], &value)
		method, path, body, key = http.MethodPost, "/api/v1/offline-copies", value, value.IdempotencyKey
	case "logs":
		if len(args) != 2 || args[1] != "cleanup" {
			err = errors.New("use logs cleanup")
			return
		}
		method, path, body = http.MethodPost, "/api/v1/logs/cleanup", map[string]string{}
	case "browse":
		if len(args) < 2 {
			err = errors.New("use browse <absolute-local-path|remote:path> [--offset 0 --limit 100 --directories]")
			return
		}
		set := flag.NewFlagSet("browse", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		offset := set.Int("offset", 0, "")
		limit := set.Int("limit", 100, "")
		directories := set.Bool("directories", false, "")
		err = set.Parse(args[2:])
		if err == nil && set.NArg() != 0 {
			err = errors.New("unexpected browse arguments")
		}
		query := url.Values{"path": {args[1]}, "offset": {fmt.Sprint(*offset)}, "limit": {fmt.Sprint(*limit)},
			"directories_only": {fmt.Sprint(*directories)}}
		path = "/api/v1/browse?" + query.Encode()
	case "tokens":
		if len(args) > 1 && args[1] == "read" {
			set := flag.NewFlagSet("tokens read", flag.ContinueOnError)
			set.SetOutput(io.Discard)
			ttl := set.Int("ttl", 3600, "")
			err = set.Parse(args[2:])
			if err == nil && set.NArg() != 0 {
				err = errors.New("unexpected token arguments")
			}
			method, path, body = http.MethodPost, "/api/v1/tokens/read", map[string]int{"ttl_seconds": *ttl}
			return
		}
		if len(args) == 4 && args[1] == "tasks" && args[2] == "--file" {
			var value json.RawMessage
			err = readCommandFile(args[3], &value)
			method, path, body = http.MethodPost, "/api/v1/tokens/tasks", value
			return
		}
		if len(args) == 3 && args[1] == "revoke" {
			method, path, body = http.MethodPost, "/api/v1/tokens/"+url.PathEscape(args[2])+"/revoke", map[string]string{}
			return
		}
		err = errors.New("use tokens read [--ttl seconds], tasks --file <grant.json>, or revoke <id>")
	case "automations":
		path = "/api/v1/automations"
		if len(args) == 1 || len(args) == 2 && args[1] == "list" {
			return
		}
		if len(args) == 3 && (args[1] == "delete" || args[1] == "trigger") {
			path += "/" + url.PathEscape(args[2])
			if args[1] == "delete" {
				method = http.MethodDelete
			} else {
				method, path, body = http.MethodPost, path+"/trigger", map[string]string{}
			}
			return
		}
		if len(args) == 4 && args[1] == "save" && args[2] == "--file" {
			var value json.RawMessage
			err = readCommandFile(args[3], &value)
			method, body = http.MethodPost, value
			return
		}
		if len(args) == 5 && args[1] == "edit" && args[3] == "--file" {
			var value json.RawMessage
			err = readCommandFile(args[4], &value)
			method, path, body = http.MethodPut, path+"/"+url.PathEscape(args[2]), value
			return
		}
		err = errors.New("use automations list, save --file <json>, edit <id> --file <json>, delete <id>, or trigger <id>")
	}
	return
}

func readCommandFile(name string, value any) error {
	raw, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return errors.New("command file must not exceed 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("command file must contain one JSON value")
	}
	return nil
}

func printAgentResult(out io.Writer, args []string, raw []byte) error {
	writer := tabwriter.NewWriter(out, 0, 4, 3, ' ', 0)
	switch args[0] {
	case "browse":
		var listing core.BrowseResult
		if err := json.Unmarshal(raw, &listing); err != nil {
			return err
		}
		fmt.Fprintf(writer, "目录\t%s\n类型\t名称\t大小\t路径\n", listing.Path)
		for _, entry := range listing.Entries {
			kind, size := "文件", formatBytes(entry.Size)
			if entry.IsDir {
				kind, size = "目录", "—"
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", kind, entry.Name, size, entry.Path)
		}
		if listing.HasMore {
			fmt.Fprintf(writer, "下一页\t--offset %d\n", listing.NextOffset)
		}
		if listing.Truncated {
			fmt.Fprintln(writer, "列表达到扫描上限；请选择更具体的目录")
		}
	case "tokens":
		if len(args) > 1 && args[1] == "revoke" {
			fmt.Fprintln(writer, "令牌已撤销")
			break
		}
		var grant struct {
			ID        string    `json:"id"`
			Token     string    `json:"token"`
			Scope     string    `json:"scope"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		if err := json.Unmarshal(raw, &grant); err != nil {
			return err
		}
		fmt.Fprintf(writer, "ID\t%s\n权限\t%s\n有效期\t%s\n令牌\t%s\n", grant.ID, grant.Scope,
			grant.ExpiresAt.Local().Format(time.RFC3339), grant.Token)
	case "automations":
		if len(args) > 1 && (args[1] == "delete" || args[1] == "trigger") {
			fmt.Fprintln(writer, "自动任务已更新")
			break
		}
		var items []core.Automation
		if len(args) > 1 && args[1] != "list" {
			var item core.Automation
			if err := json.Unmarshal(raw, &item); err != nil {
				return err
			}
			items = []core.Automation{item}
		} else if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		fmt.Fprintln(writer, "ID\t名称\t类型\t启用\t状态\t下次运行")
		for _, item := range items {
			next := "—"
			if item.NextRunAt != nil {
				next = item.NextRunAt.Local().Format(time.RFC3339)
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%s\t%s\n", item.ID, item.Name, item.Kind, item.Enabled, item.Status, next)
		}
	}
	return writer.Flush()
}
