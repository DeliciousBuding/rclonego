package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
)

func setupRequest(args []string) (method, path string, body any, key string, err error) {
	method = http.MethodGet
	switch args[0] {
	case "engine":
		if len(args) == 1 || len(args) == 2 && args[1] == "status" {
			path = "/api/v1/engine"
			return
		}
		if len(args) == 2 && args[1] == "install" {
			method, path, body = http.MethodPost, "/api/v1/engine/install", map[string]string{}
			return
		}
		if len(args) == 4 && args[1] == "install" && args[2] == "--archive" {
			method, path, body = http.MethodPost, "/api/v1/engine/install", map[string]string{"archive": args[3]}
			return
		}
		if len(args) == 3 && args[1] == "select" {
			method, path, body = http.MethodPost, "/api/v1/engine/select", map[string]string{"path": args[2]}
			return
		}
	case "accounts":
		if len(args) == 3 && args[1] == "quota" {
			path = "/api/v1/remotes/" + url.PathEscape(args[2]) + "/quota"
			return
		}
		if len(args) == 3 && args[1] == "info" {
			path = "/api/v1/remotes/" + url.PathEscape(args[2]) + "/info"
			return
		}
		if len(args) == 2 && args[1] == "providers" {
			path = "/api/v1/providers"
			return
		}
		if len(args) == 3 && args[1] == "show" {
			path = "/api/v1/accounts/" + url.PathEscape(args[2])
			return
		}
		if len(args) == 3 && args[1] == "cancel" {
			method, path, body = http.MethodPost, "/api/v1/accounts/"+url.PathEscape(args[2])+"/cancel", map[string]string{}
			return
		}
		if len(args) == 4 && args[1] == "create" && args[2] == "--file" {
			var raw []byte
			if args[3] == "-" {
				raw, err = io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
			} else {
				raw, err = os.ReadFile(args[3])
			}
			if err != nil {
				return
			}
			var value json.RawMessage
			err = json.Unmarshal(raw, &value)
			method, path, body = http.MethodPost, "/api/v1/accounts", value
			return
		}
		if len(args) == 4 && args[1] == "answer" && args[3] == "--stdin" {
			var raw []byte
			raw, err = io.ReadAll(io.LimitReader(os.Stdin, 64<<10))
			var value struct {
				Answer string `json:"answer"`
			}
			if err == nil {
				err = json.Unmarshal(raw, &value)
			}
			method, path, body = http.MethodPost, "/api/v1/accounts/"+url.PathEscape(args[2])+"/answer", value
			return
		}
	}
	err = errors.New("use engine status|install [--archive path]|select path; accounts quota NAME|info NAME|providers|create --file path|show ID|answer ID --stdin|cancel ID")
	return
}
