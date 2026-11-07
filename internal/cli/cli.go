package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/internal/session"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

const appVersion = "0.1.0-dev"

const cliHelp = `RcloneGo — 本机数据工作台

用法：
  rclonego                         打开原生客户端
  rclonego serve [启动参数]          只运行本机 API 和任务队列
  rclonego <命令> [选项]

命令：
  status                                   查看上传和队列状态
  jobs [list]                              列出任务
  jobs show <任务ID>                        查看一个任务的完整状态
  jobs add --name <名称> --source <源> --destination <目标>
           [--operation copy|copyto|sync|check] [--transfers 2] [--checkers 4]
           [--bandwidth 4M] [--checksum] [--allow-delete]
           [--chunk-size 16M] [--buffer-size 8M] [--cpu-parallelism 2]
           [--include <规则>] [--exclude <规则>] [--immutable]
           [--at <RFC3339时间>] [--idempotency-key <请求标识>]
  jobs add --file <任务.json>               从结构化请求创建任务
  jobs retry|cancel|delete <任务ID>          操作任务
  jobs edit <任务ID> --file <参数.json>      调整等待任务的参数和计划时间
  jobs up|down <任务ID>                     调整等待顺序
  jobs log <任务ID>                         查看日志
  mounts [list]                            列出盘符映射
  mounts start|stop <映射ID>                 挂载或卸载
  remotes [list]                           列出云端账户
  remotes reconnect <远端名称>              创建重新授权任务
  queue pause|resume                       暂停或继续队列调度
  theme [light|dark|system]                 查看或切换外观
  settings                                查看持久化默认参数
  settings --file <设置.json>              保存 appearance 或 defaults
  external --file <配置.json>              配置现有上传器的 RC 与所有者
  resources                               查看进程资源和线程
  bandwidth <任务ID或外部ID> <4M|off>        调整运行限速
  workflows list|save --file <json>|run <ID>  保存或执行工作流定义
  runs list|report|wait|watch <ID>            执行记录和校验结果
  doctor                                    引擎与平台诊断
  workflow --name <名称> --source <本机根目录> --destination <目标>
           --idempotency-key <标识> [--proof <排除证明.json>]
  api                                      查看 API 能力

通用选项：
  --api <URL>       默认发现本机实例，可用 RCLONEGO_API 设置
  --json            输出 JSON，便于 Agent 和脚本处理
  --timeout <时长>  默认 15s
  --help            查看命令说明
  --version         查看版本

任务默认复制；同步删除必须显式开启 --allow-delete。
`

type cliOptions struct {
	api     string
	json    bool
	timeout time.Duration
}

func IsCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "status", "jobs", "mounts", "remotes", "queue", "theme", "api", "help", "--help", "-h", "--version":
		return true
	case "resources", "bandwidth", "workflow", "workflows", "runs", "doctor", "settings", "external":
		return true
	case "--api", "--json", "--timeout":
		return true
	}
	return false
}

func Run(args []string, stdout, stderr io.Writer) int {
	options := cliOptions{api: os.Getenv("RCLONEGO_API"), timeout: 15 * time.Second}
	command, err := parseCLIOptions(args, &options)
	if err != nil {
		return cliError(stderr, options.json, err)
	}
	if len(command) == 0 || command[0] == "help" || contains(command, "--help") || contains(command, "-h") {
		fmt.Fprint(stdout, cliHelp)
		return 0
	}
	if command[0] == "--version" {
		fmt.Fprintln(stdout, appVersion)
		return 0
	}
	if options.api == "" {
		dir, err := session.DataDir()
		if err != nil {
			return cliError(stderr, options.json, err)
		}
		if override := os.Getenv("RCLONEGO_DATA"); override != "" {
			dir = override
		}
		info, _, err := session.Discover(dir)
		if err != nil {
			return cliError(stderr, options.json, errors.New("no running instance; start rclonego or rclonego serve"))
		}
		options.api = info.API
	}
	endpoint, err := url.Parse(options.api)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return cliError(stderr, options.json, errors.New("--api must be an HTTP URL"))
	}
	if !loopbackHost(endpoint.Hostname()) {
		return cliError(stderr, options.json, errors.New("--api must point to localhost"))
	}
	method, path, body, key, err := cliRequest(command)
	if err != nil {
		return cliError(stderr, options.json, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()
	if len(command) == 3 && command[0] == "runs" && (command[1] == "wait" || command[1] == "watch") {
		return waitRun(ctx, options, command, stdout, stderr)
	}
	raw, err := callAPI(ctx, options.api, method, path, body, key)
	if err != nil {
		return cliError(stderr, options.json, err)
	}
	if options.json {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			return cliError(stderr, true, err)
		}
		fmt.Fprintln(stdout, pretty.String())
		return 0
	}
	if err := printCLIResult(stdout, command, raw); err != nil {
		return cliError(stderr, false, err)
	}
	return 0
}

func parseCLIOptions(args []string, options *cliOptions) ([]string, error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, inline := strings.Cut(arg, "=")
		switch name {
		case "--json":
			if inline {
				return nil, errors.New("--json does not take a value")
			}
			options.json = true
		case "--api", "--timeout":
			if !inline {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("%s requires a value", name)
				}
				value = args[i]
			}
			if name == "--api" {
				options.api = value
			} else {
				duration, err := time.ParseDuration(value)
				if err != nil || duration <= 0 {
					return nil, errors.New("--timeout must be a positive duration")
				}
				options.timeout = duration
			}
		default:
			rest = append(rest, arg)
		}
	}
	return rest, nil
}

func loopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func cliRequest(args []string) (method, path string, body any, key string, err error) {
	if args[0] == "workflows" || args[0] == "runs" || args[0] == "doctor" {
		return executionRequest(args)
	}
	method = http.MethodGet
	exact := func(n int) bool {
		if len(args) != n {
			err = fmt.Errorf("invalid arguments for %s; use --help", args[0])
			return false
		}
		return true
	}
	switch args[0] {
	case "settings", "external":
		path = "/api/v1/settings"
		if args[0] == "external" {
			path = "/api/v1/external"
		}
		if len(args) == 1 && args[0] == "settings" {
			break
		}
		if len(args) != 3 || args[1] != "--file" {
			err = fmt.Errorf("%s requires --file <JSON>", args[0])
			break
		}
		raw, readErr := os.ReadFile(args[2])
		err = readErr
		if err == nil {
			var value json.RawMessage
			err = json.Unmarshal(raw, &value)
			body = value
			method = http.MethodPut
		}
	case "workflow":
		set := flag.NewFlagSet("workflow", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		request := core.WorkflowRequest{}
		set.StringVar(&request.Name, "name", "", "")
		set.StringVar(&request.Source, "source", "", "")
		set.StringVar(&request.Destination, "destination", "", "")
		set.StringVar(&request.IdempotencyKey, "idempotency-key", "", "")
		set.StringVar(&request.ProofPath, "proof", "", "")
		var entries repeatedOption
		set.Var(&entries, "entry", "")
		err = set.Parse(args[1:])
		if err == nil && set.NArg() != 0 {
			err = errors.New("unexpected workflow arguments")
		}
		request.Entries = entries
		method = http.MethodPost
		path = "/api/v1/workflows"
		body = request
	case "status":
		if exact(1) {
			path = "/api/v1/overview"
		}
	case "api":
		if exact(1) {
			path = "/api/v1"
		}
	case "resources":
		if exact(1) {
			path = "/api/v1/runtime?view=resources"
		}
	case "bandwidth":
		if exact(3) {
			method = http.MethodPost
			path = "/api/v1/runtime/bandwidth"
			body = map[string]string{"id": args[1], "bandwidth": args[2]}
		}
	case "jobs":
		if len(args) == 1 || (len(args) == 2 && args[1] == "list") {
			path = "/api/v1/jobs"
			break
		}
		if len(args) >= 2 && args[1] == "add" {
			request, idempotency, parseErr := parseJobCLI(args[2:])
			err = parseErr
			method = http.MethodPost
			path = "/api/v1/jobs"
			body = request
			key = idempotency
			break
		}
		if len(args) == 5 && args[1] == "edit" && args[3] == "--file" {
			raw, readErr := os.ReadFile(args[4])
			err = readErr
			if err == nil {
				var request core.JobUpdateRequest
				err = json.Unmarshal(raw, &request)
				body = request
			}
			method, path = http.MethodPut, "/api/v1/jobs/"+url.PathEscape(args[2])
			break
		}
		if !exact(3) {
			break
		}
		switch args[1] {
		case "show":
			path = "/api/v1/jobs/" + url.PathEscape(args[2])
		case "log":
			path = "/api/v1/jobs/log?id=" + url.QueryEscape(args[2])
		case "retry", "cancel", "delete":
			method = http.MethodPost
			path = "/api/v1/jobs/action"
			body = map[string]string{"id": args[2], "action": args[1]}
		case "up", "down":
			method = http.MethodPost
			path = "/api/v1/jobs/action"
			delta := -1
			if args[1] == "down" {
				delta = 1
			}
			body = map[string]any{"id": args[2], "action": "move", "delta": delta}
		default:
			err = errors.New("jobs accepts list, add, retry, cancel, delete, up, down or log")
		}
	case "mounts":
		if len(args) == 1 || (len(args) == 2 && args[1] == "list") {
			path = "/api/v1/mounts"
			break
		}
		if !exact(3) {
			break
		}
		if args[1] != "start" && args[1] != "stop" {
			err = errors.New("mounts accepts list, start or stop")
			break
		}
		method = http.MethodPost
		path = "/api/v1/mounts/" + args[1]
		body = map[string]string{"id": args[2]}
	case "remotes":
		if len(args) == 1 || (len(args) == 2 && args[1] == "list") {
			path = "/api/v1/remotes"
			break
		}
		if !exact(3) {
			break
		}
		if args[1] != "reconnect" {
			err = errors.New("remotes accepts list or reconnect")
			break
		}
		method = http.MethodPost
		path = "/api/v1/remotes/reconnect"
		body = map[string]string{"name": args[2]}
	case "queue":
		if !exact(2) {
			break
		}
		if args[1] != "pause" && args[1] != "resume" {
			err = errors.New("queue accepts pause or resume")
			break
		}
		method = http.MethodPost
		path = "/api/v1/queue/pause"
		body = map[string]bool{"paused": args[1] == "pause"}
	case "theme":
		path = "/api/v1/settings"
		if len(args) == 1 {
			break
		}
		if !exact(2) {
			break
		}
		if !core.ValidAppearance(args[1]) {
			err = errors.New("theme accepts light, dark or system")
			break
		}
		method = http.MethodPut
		body = map[string]string{"appearance": args[1]}
	default:
		err = fmt.Errorf("unknown command %q; use --help", args[0])
	}
	return
}

type repeatedOption []string

func (r *repeatedOption) String() string         { return strings.Join(*r, ",") }
func (r *repeatedOption) Set(value string) error { *r = append(*r, value); return nil }

func parseJobCLI(args []string) (core.JobCreateRequest, string, error) {
	request := core.JobCreateRequest{}
	set := flag.NewFlagSet("jobs add", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	file := set.String("file", "", "")
	key := set.String("idempotency-key", "", "")
	set.StringVar(&request.Name, "name", "", "")
	set.StringVar(&request.Operation, "operation", "copy", "")
	set.StringVar(&request.Source, "source", "", "")
	set.StringVar(&request.Destination, "destination", "", "")
	set.IntVar(&request.Transfers, "transfers", 0, "")
	set.IntVar(&request.Checkers, "checkers", 0, "")
	set.StringVar(&request.Bandwidth, "bandwidth", "", "")
	set.BoolVar(&request.Checksum, "checksum", false, "")
	set.BoolVar(&request.AllowDelete, "allow-delete", false, "")
	set.BoolVar(&request.Immutable, "immutable", false, "")
	set.StringVar(&request.ChunkSize, "chunk-size", "", "")
	set.StringVar(&request.BufferSize, "buffer-size", "", "")
	cpu := set.Int("cpu-parallelism", 0, "")
	set.BoolVar(&request.CreateDirs, "create-empty-dirs", false, "")
	at := set.String("at", "", "")
	var includes repeatedOption
	var excludes repeatedOption
	set.Var(&includes, "include", "")
	set.Var(&excludes, "exclude", "")
	if err := set.Parse(args); err != nil {
		return request, "", err
	}
	if set.NArg() != 0 {
		return request, "", errors.New("unexpected arguments to jobs add")
	}
	if *file != "" {
		conflict := false
		set.Visit(func(f *flag.Flag) {
			if f.Name != "file" && f.Name != "idempotency-key" {
				conflict = true
			}
		})
		if conflict {
			return request, "", errors.New("--file cannot be combined with task fields")
		}
		data, err := os.ReadFile(*file)
		if err != nil {
			return request, "", err
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&request); err != nil {
			return request, "", err
		}
		if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return request, "", errors.New("task file must contain one JSON value")
		}
	} else {
		request.Includes = includes
		request.Excludes = excludes
		set.Visit(func(f *flag.Flag) {
			if f.Name == "cpu-parallelism" {
				request.CPUParallelism = cpu
			}
		})
	}
	if request.Name == "" || request.Source == "" || request.Destination == "" {
		return request, "", errors.New("jobs add requires --name, --source and --destination")
	}
	if *at != "" {
		when, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			return request, "", errors.New("--at requires an RFC3339 timestamp with timezone")
		}
		request.NotBefore = &when
	}
	return request, *key, nil
}

func callAPI(ctx context.Context, base, method, path string, body any, key string) ([]byte, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RcloneGo-Client", "1")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	tokenFile := os.Getenv("RCLONEGO_TOKEN_FILE")
	if tokenFile == "" {
		dir, err := session.DataDir()
		if err == nil {
			if override := os.Getenv("RCLONEGO_DATA"); override != "" {
				dir = override
			}
			tokenFile = filepath.Join(dir, "api-token")
		}
	}
	if rawToken, err := os.ReadFile(tokenFile); err == nil {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(rawToken)))
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %w", base, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var message struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &message)
		return nil, fmt.Errorf("%s: %s", resp.Status, core.ValueOr(message.Error, strings.TrimSpace(string(raw))))
	}
	return raw, nil
}

func printCLIResult(out io.Writer, args []string, raw []byte) error {
	if args[0] == "workflows" || args[0] == "runs" || args[0] == "doctor" {
		return printExecution(out, args, raw)
	}
	w := tabwriter.NewWriter(out, 0, 4, 3, ' ', 0)
	switch args[0] {
	case "resources":
		var snapshot core.ResourceSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return err
		}
		fmt.Fprintf(w, "采样时间\t%s\n逻辑核心\t%d\n客户端协程\t%d\n", snapshot.UpdatedAt.Local().Format("2006-01-02 15:04:05"), snapshot.LogicalCPUs, snapshot.Goroutines)
		fmt.Fprintln(w, "PID\t进程\tCPU\t私有提交内存\t系统线程")
		for _, p := range snapshot.Processes {
			cpu, memory := "—", "—"
			if p.CPUAvailable {
				cpu = fmt.Sprintf("%.1f%%", p.CPUPercent)
			}
			if p.MemoryAvailable {
				memory = formatBytes(p.MemoryBytes)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n", p.PID, p.Name, cpu, memory, p.Threads)
		}
	case "workflow":
		var jobs []core.Job
		if err := json.Unmarshal(raw, &jobs); err != nil {
			return err
		}
		fmt.Fprintf(w, "已创建工作流\t%d 个步骤\n", len(jobs))
		for _, job := range jobs {
			fmt.Fprintf(w, "%d/%d\t%s\t%s\t%s\n", job.WorkflowStep, job.WorkflowSize, job.ID, operationLabel(job.Operation), job.Name)
		}
	case "bandwidth":
		fmt.Fprintf(w, "运行限速\t%s\n", args[2])
	case "status":
		var state struct {
			Version     string              `json:"version"`
			Activity    core.ActivityStatus `json:"activity"`
			Paused      bool                `json:"queue_paused"`
			Jobs        []core.Job          `json:"jobs"`
			RemoteCount int                 `json:"remote_count"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			return err
		}
		fmt.Fprintf(w, "rclone\t%s\n云端账户\t%d\n队列\t%s · %d 个任务\n", state.Version, state.RemoteCount, map[bool]string{true: "已暂停", false: "就绪"}[state.Paused], len(state.Jobs))
		if state.Activity.Available && state.Activity.Stats != nil {
			s := state.Activity.Stats
			pct := progressValue(s.Bytes, s.TotalBytes)
			if pct >= 0 {
				fmt.Fprintf(w, "当前阶段\t%s / %s · %.1f%%\n", formatBytes(s.Bytes), formatBytes(s.TotalBytes), pct*100)
			}
			fmt.Fprintf(w, "上传速度\t%.2f MiB/s\n活动文件\t%d\n已传文件\t%d / %d\n错误\t%d\n", s.Speed/1024/1024, len(s.Transferring), s.Transfers, s.TotalTransfers, s.Errors)
			if len(s.Transferring) > 0 {
				fmt.Fprintf(w, "目标\t%s\n", s.Transferring[0].DstFs)
			}
		} else {
			fmt.Fprintln(w, "上传器\t未连接")
		}
	case "jobs":
		if len(args) == 3 && args[1] == "log" {
			var result struct {
				Log string `json:"log"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			fmt.Fprint(out, result.Log)
			return nil
		}
		if len(args) == 3 && args[1] == "show" {
			var job core.Job
			if err := json.Unmarshal(raw, &job); err != nil {
				return err
			}
			fmt.Fprintf(w, "ID\t%s\n任务\t%s\n状态\t%s\n源\t%s\n目标\t%s\n", job.ID, job.Name, statusLabel(job.Status), job.Source, job.Destination)
			fmt.Fprintf(w, "并发\t%d 传输 / %d 校验\n缓冲\t%s\n分块\t%s\nCPU 并行度\t%d\n", job.Transfers, job.Checkers, job.BufferSize, job.ChunkSize, job.CPUParallelism)
			fmt.Fprintf(w, "字节\t%s / %s\n文件\t%d / %d\n校验\t%d / %d\n", formatBytes(job.Bytes), formatBytes(job.TotalBytes), job.FilesDone, job.FilesTotal, job.ChecksDone, job.ChecksTotal)
			if job.Error != "" {
				fmt.Fprintf(w, "错误\t%s\n", job.Error)
			}
			return w.Flush()
		}
		if len(args) == 1 || (len(args) == 2 && args[1] == "list") {
			var result struct {
				Jobs []core.Job `json:"jobs"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			fmt.Fprintln(w, "顺序\tID\t任务\t状态\t操作\t已传")
			sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].Position < result.Jobs[j].Position })
			for _, job := range result.Jobs {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", job.Position, job.ID, job.Name, statusLabel(job.Status), operationLabel(job.Operation), formatBytes(job.Bytes))
			}
		} else {
			var job core.Job
			_ = json.Unmarshal(raw, &job)
			if job.ID != "" {
				fmt.Fprintf(w, "已创建\t%s\t%s\n", job.ID, job.Name)
			} else {
				fmt.Fprintln(w, "任务操作完成")
			}
		}
	case "mounts":
		if len(args) <= 2 {
			var mounts []core.MountView
			if err := json.Unmarshal(raw, &mounts); err != nil {
				return err
			}
			fmt.Fprintln(w, "ID\t盘符\t名称\t远端\t状态")
			for _, m := range mounts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.ID, m.Drive, m.Name, m.Remote, map[bool]string{true: "已挂载", false: "未挂载"}[m.Running])
			}
		} else {
			fmt.Fprintln(w, "映射操作完成")
		}
	case "remotes":
		if len(args) <= 2 {
			var remotes []core.RemoteInfo
			if err := json.Unmarshal(raw, &remotes); err != nil {
				return err
			}
			fmt.Fprintln(w, "名称\t类型")
			for _, r := range remotes {
				fmt.Fprintf(w, "%s\t%s\n", r.Name, r.Type)
			}
		} else {
			var job core.Job
			_ = json.Unmarshal(raw, &job)
			fmt.Fprintf(w, "授权任务已创建\t%s\n", job.ID)
		}
	case "theme":
		var result struct {
			Appearance string `json:"appearance"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return err
		}
		fmt.Fprintf(w, "外观\t%s\n", result.Appearance)
	case "queue":
		fmt.Fprintln(w, "队列已"+map[string]string{"pause": "暂停", "resume": "继续"}[args[1]])
	default:
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			return err
		}
		fmt.Fprintln(w, pretty.String())
	}
	return w.Flush()
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cliError(out io.Writer, structured bool, err error) int {
	if structured {
		_ = json.NewEncoder(out).Encode(map[string]any{"ok": false, "error": err.Error()})
	} else {
		fmt.Fprintln(out, "错误：", err)
	}
	return 1
}
