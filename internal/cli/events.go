package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func watchEvents(ctx context.Context, options cliOptions, endpoint string, out, stderr io.Writer) int {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(options.api, "/")+endpoint, nil)
	if err != nil {
		return cliError(stderr, options.json, err)
	}
	request.Header.Set("Accept", "text/event-stream")
	authenticateCLIRequest(request)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return cliError(stderr, options.json, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		var failure APIError
		_ = json.Unmarshal(raw, &failure)
		failure.HTTPStatus = response.StatusCode
		if failure.Message == "" {
			failure.Message = response.Status
		}
		return cliError(stderr, options.json, &failure)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return cliError(stderr, options.json, errors.New("event watch requires a text/event-stream response"))
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	kind, cursor, data := "", "", ""
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			kind = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "id: "):
			cursor = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && data != "":
			if kind == "error" {
				var failure APIError
				_ = json.Unmarshal([]byte(data), &failure)
				failure.HTTPStatus = map[string]int{"cursor_expired": 410, "authentication_required": 401}[failure.Code]
				return cliError(stderr, options.json, &failure)
			}
			if options.json {
				sequence, err := strconv.ParseInt(cursor, 10, 64)
				if err != nil || sequence < 0 {
					return cliError(stderr, true, errors.New("event stream returned an invalid cursor"))
				}
				if err := json.NewEncoder(out).Encode(map[string]any{
					"type": kind, "cursor": sequence, "data": json.RawMessage(data),
				}); err != nil {
					return cliError(stderr, true, err)
				}
			} else if kind == "event" {
				var event struct {
					OccurredAt string `json:"occurred_at"`
					Subject    string `json:"subject"`
					Kind       string `json:"kind"`
				}
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					return cliError(stderr, false, err)
				}
				fmt.Fprintf(out, "%s  %s  %s  %s\n", cursor, event.OccurredAt, event.Kind, event.Subject)
			}
			kind, cursor, data = "", "", ""
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 3
		}
		return cliError(stderr, options.json, err)
	}
	return 0
}
