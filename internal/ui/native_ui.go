package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type nativeUI struct {
	app  *core.App
	win  *mygo.Window
	page string
	api  string

	version            string
	remotes            []core.RemoteInfo
	mounts             []core.MountView
	jobs               []core.Job
	activity           core.ActivityStatus
	external           []core.ExternalTransferView
	resources          core.ResourceSnapshot
	queueBlockReason   string
	defaults           core.TransferSettings
	treeOpen           map[string]*bool
	historyOpen        bool
	waitingOpen        bool
	paused             bool
	busy               bool
	notice             string
	isError            bool
	appearance         string
	appearanceChoice   string
	jobOperationChoice string
	remoteTypeChoice   string
	noticeUntil        time.Time
	lastUpdated        time.Time
	activitySpeeds     []float64
	showRemoteForm     bool
	showJobAdvanced    bool
	showMountAdvanced  bool

	newRemoteName string
	newRemoteType string

	editingMount bool
	mountID      string
	mountName    string
	mountRemote  string
	mountDrive   string
	mountVolume  string
	mountCache   string
	mountMax     string
	mountAge     string

	jobName             string
	jobOperation        string
	jobSource           string
	jobDestination      string
	jobIncludes         string
	jobTransfers        string
	jobCheckers         string
	jobBandwidth        string
	jobChunk            string
	jobBuffer           string
	jobSchedule         string
	liveBandwidth       string
	liveTarget          string
	defaultTransfers    string
	defaultCheckers     string
	defaultChunk        string
	defaultBuffer       string
	defaultBandwidth    string
	defaultCPU          string
	jobCPU              string
	showDefaults        bool
	jobChecksum         bool
	jobCreateDirs       bool
	jobAllowDelete      bool
	jobSelected         int
	jobTable            ui.ListState
	logJobID            string
	logText             string
	jobSpeeds           map[string][]float64
	showJobForm         bool
	editingJobID        string
	jobWorkflow         bool
	jobProof            string
	saveDefinition      bool
	workflows           []core.WorkflowDefinition
	runs                []core.Run
	runReport           string
	enginePath          string
	engineArchive       string
	providers           []core.Provider
	account             core.AccountView
	accountAnswer       string
	accountClientID     string
	accountClientSecret string
	openAtLogin         bool
	trayAvailable       bool
	jobPreview          core.Preview
	previewOnly         bool
}

func New(app *core.App, api string) *nativeUI {
	defaults := app.Snapshot().Defaults
	return &nativeUI{
		app: app, page: "overview", api: api, newRemoteType: "drive", appearance: app.Snapshot().Appearance,
		appearanceChoice: appearanceLabel(app.Snapshot().Appearance),
		jobOperation:     "copy", jobOperationChoice: "复制", remoteTypeChoice: "Google Drive",
		jobTransfers: itoa(defaults.Transfers), jobCheckers: itoa(defaults.Checkers), jobChunk: defaults.ChunkSize, jobBuffer: defaults.BufferSize,
		jobCPU: itoa(defaults.CPUParallelism), defaults: defaults,
		defaultTransfers: itoa(defaults.Transfers), defaultCheckers: itoa(defaults.Checkers),
		defaultChunk: defaults.ChunkSize, defaultBuffer: defaults.BufferSize, defaultBandwidth: defaults.Bandwidth,
		defaultCPU:  itoa(defaults.CPUParallelism),
		jobSelected: -1, jobTable: ui.ListState{}, jobSpeeds: map[string][]float64{},
		treeOpen: map[string]*bool{}, waitingOpen: true,
	}
}

func (s *nativeUI) Attach(win *mygo.Window) {
	s.win = win
}

func (s *nativeUI) StartPolling(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		s.refresh(ctx, true)
		tick := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tick++
				s.refresh(ctx, tick%4 == 0)
			}
		}
	}()
}

func (s *nativeUI) refresh(ctx context.Context, includeRemoteState bool) {
	if ctx.Err() != nil {
		return
	}
	if s.win == nil {
		return
	}
	s.win.Update(func() {
		if s.account.ID != "" {
			if account, err := s.app.Account(s.account.ID); err == nil {
				s.account = account
			}
		}
	})
	state := s.app.Snapshot()
	runtime := s.app.RuntimeSnapshot()
	activity := core.ActivityStatus{}
	for _, external := range runtime.External {
		if external.Available {
			activity = core.ActivityStatus{Available: true, Stats: external.Stats}
			break
		}
	}
	if !activity.Available {
		for _, job := range state.Jobs {
			if job.Status == "running" {
				activity = core.ActivityStatus{Available: true, Stats: &core.RCStats{
					Bytes: job.Bytes, TotalBytes: job.TotalBytes, Speed: job.Speed,
					Transfers: job.FilesDone, TotalTransfers: job.FilesTotal,
					Checks: job.ChecksDone, TotalChecks: job.ChecksTotal,
					Transferring: job.ActiveFiles, Checking: job.CheckingFiles,
				}}
				break
			}
		}
	}
	jobs := append([]core.Job(nil), state.Jobs...)
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if s.win == nil {
		return
	}
	s.win.Update(func() {
		if s.appearance != state.Appearance {
			s.appearance = state.Appearance
			s.appearanceChoice = appearanceLabel(state.Appearance)
			mygo.Theme.SetSource(mygo.ThemeSource(s.appearance))
		}
		s.remotes, s.mounts, s.version = runtime.Remotes, runtime.Mounts, runtime.Version
		s.defaults = state.Defaults
		if !s.showDefaults {
			d := state.Defaults
			s.defaultTransfers, s.defaultCheckers, s.defaultCPU = itoa(d.Transfers), itoa(d.Checkers), itoa(d.CPUParallelism)
			s.defaultChunk, s.defaultBuffer, s.defaultBandwidth = d.ChunkSize, d.BufferSize, d.Bandwidth
		}
		if includeRemoteState && runtime.CatalogError != "" && runtime.Version != "" {
			s.setError(fmt.Errorf("%s", runtime.CatalogError))
		}
		s.jobs, s.paused, s.activity = jobs, state.QueuePaused, activity
		s.workflows, s.runs = state.Workflows, state.Runs
		s.external, s.resources = runtime.External, runtime.Resources
		s.queueBlockReason = runtime.QueueBlockReason
		s.pruneTreeState()
		s.lastUpdated = time.Now()
		if activity.Available && activity.Stats != nil {
			s.activitySpeeds = append(s.activitySpeeds, activity.Stats.Speed)
			if len(s.activitySpeeds) > 60 {
				s.activitySpeeds = s.activitySpeeds[len(s.activitySpeeds)-60:]
			}
		}
		for _, job := range jobs {
			if job.Status != "running" || job.Speed <= 0 {
				continue
			}
			history := append(s.jobSpeeds[job.ID], job.Speed)
			if len(history) > 30 {
				history = history[len(history)-30:]
			}
			s.jobSpeeds[job.ID] = history
		}
		if s.jobSelected >= len(s.jobs) {
			s.jobSelected = len(s.jobs) - 1
		}
		for id := range s.jobSpeeds {
			found := false
			for _, job := range jobs {
				if job.ID == id {
					found = true
					break
				}
			}
			if !found {
				delete(s.jobSpeeds, id)
			}
		}
	})
}

func (s *nativeUI) setError(err error) {
	if err == nil {
		return
	}
	s.isError = true
	s.notice = err.Error()
	s.noticeUntil = time.Time{}
}

func (s *nativeUI) run(label string, action func() error) {
	s.runAfter(label, action, nil)
}

func (s *nativeUI) runAfter(label string, action func() error, success func()) {
	if s.busy {
		return
	}
	s.busy, s.isError, s.notice = true, false, label
	s.invalidate()
	go func() {
		err := action()
		s.win.Update(func() {
			s.busy = false
			if err != nil {
				s.isError, s.notice = true, err.Error()
			} else {
				s.isError, s.notice = false, label+"完成"
				s.noticeUntil = time.Now().Add(4 * time.Second)
				if success != nil {
					success()
				}
			}
		})
		s.app.RefreshRuntime()
		go s.refresh(context.Background(), true)
	}()
}

func (s *nativeUI) mountsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Gap(12).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 个映射", len(s.mounts))).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.PrimaryButton(c.Key("new-mount"), "添加映射…").Height(30).Disabled(s.busy).OnClick(func() {
				s.beginMountEdit(core.MountConfig{})
			})
		})
		contentScroll(c).Grow(1).Gap(0).Children(func() {
			for _, mount := range s.mounts {
				s.mountRow(c, mount)
			}
		})
	})
}

func (s *nativeUI) mountRow(c *ui.Context, mount core.MountView) {
	t := c.Theme()
	ui.Row(c).Padding(13, 0).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Box(c).Size(42, 34).Radius(8).Center().Background(t.Accent.Alpha(0.10)).Children(func() {
			ui.Text(c, mount.Drive).FontSize(14).Bold().TextColor(t.Accent)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, mount.Name).Bold().SingleLine().Tooltip(mount.Name)
			ui.Text(c, mount.Remote).FontSize(12).TextColor(t.TextMuted).SingleLine()
		})
		if mount.Running {
			statusPill(c, "运行中", t.Success)
			ui.Button(c.Key("stop-"+mount.ID), "卸载").Padding(6, 10).Tooltip("卸载此盘符").OnClick(func() {
				cfg := mount.MountConfig
				s.run("卸载 "+cfg.Drive, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					return s.app.StopMount(ctx, cfg)
				})
			}).Disabled(s.busy)
		} else {
			ui.Button(c.Key("start-"+mount.ID), "挂载").Padding(6, 10).Tooltip("挂载此盘符").OnClick(func() {
				cfg := mount.MountConfig
				s.run("挂载 "+cfg.Drive, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					_, err := s.app.StartMount(ctx, cfg)
					return err
				})
			}).Disabled(s.busy)
			ui.Button(c.Key("delete-"+mount.ID), "删除").Padding(6, 10).TextColor(t.Danger).
				Tooltip("删除此映射配置").OnClick(func() {
				id := mount.ID
				s.run("删除映射", func() error {
					return s.app.UpdateState(func(state *core.State) error {
						for i, cfg := range state.Mounts {
							if cfg.ID == id {
								state.Mounts = append(state.Mounts[:i], state.Mounts[i+1:]...)
								return nil
							}
						}
						return fmt.Errorf("mount %s not found", id)
					})
				})
			}).Disabled(s.busy)
		}
		ui.Button(c.Key("edit-"+mount.ID), "编辑…").Height(28).Padding(4, 10).OnClick(func() { s.beginMountEdit(mount.MountConfig) }).Disabled(s.busy)
	})
}

func (s *nativeUI) beginMountEdit(cfg core.MountConfig) {
	s.editingMount = true
	s.mountID, s.mountName, s.mountRemote = cfg.ID, cfg.Name, cfg.Remote
	s.mountDrive, s.mountVolume, s.mountCache = cfg.Drive, cfg.VolumeName, cfg.CacheDir
	s.mountMax, s.mountAge = core.ValueOr(cfg.CacheMax, "15G"), core.ValueOr(cfg.CacheMaxAge, "24h")
	s.invalidate()
}

func (s *nativeUI) addJob() {
	transfers, err := strconv.Atoi(s.jobTransfers)
	if err != nil {
		s.isError, s.notice = true, "transfers 必须是数字"
		s.invalidate()
		return
	}
	checkers, err := strconv.Atoi(s.jobCheckers)
	if err != nil {
		s.isError, s.notice = true, "checkers 必须是数字"
		s.invalidate()
		return
	}
	request := core.JobCreateRequest{
		Name: s.jobName, Operation: s.jobOperation, Source: s.jobSource,
		Destination: s.jobDestination, Transfers: transfers, Checkers: checkers,
		Bandwidth: s.jobBandwidth, Checksum: s.jobChecksum,
		CreateDirs: s.jobCreateDirs, AllowDelete: s.jobAllowDelete,
		ChunkSize: s.jobChunk, BufferSize: s.jobBuffer,
		PreviewID: s.jobPreview.ID,
	}
	cpus, err := strconv.Atoi(s.jobCPU)
	if err != nil {
		s.setError(fmt.Errorf("CPU 并行度必须是数字"))
		return
	}
	request.CPUParallelism = &cpus
	if strings.TrimSpace(s.jobSchedule) != "" {
		at, err := time.ParseInLocation("2006-01-02 15:04", s.jobSchedule, time.Local)
		if err != nil {
			s.setError(fmt.Errorf("计划时间格式为 YYYY-MM-DD HH:MM"))
			return
		}
		request.NotBefore = &at
	}
	for _, line := range strings.Split(s.jobIncludes, "\n") {
		if value := strings.TrimSpace(line); value != "" {
			request.Includes = append(request.Includes, value)
		}
	}
	if s.previewOnly || request.Operation == "sync" && s.jobPreview.ID == "" {
		s.previewOnly = false
		var preview core.Preview
		s.runAfter("预览更改", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			var err error
			preview, err = s.app.PreviewJob(ctx, request)
			return err
		}, func() { s.jobPreview = preview })
		return
	}
	editID, workflow, proof := s.editingJobID, s.jobWorkflow, s.jobProof
	saveDefinition := s.saveDefinition
	s.runAfter("保存任务", func() error {
		settings := core.TransferSettings{Transfers: transfers, Checkers: checkers, ChunkSize: request.ChunkSize, BufferSize: request.BufferSize, CPUParallelism: cpus, Bandwidth: request.Bandwidth}
		if editID != "" {
			return s.app.UpdateParameters(editID, core.JobUpdateRequest{Settings: settings, NotBefore: request.NotBefore})
		}
		if saveDefinition {
			_, err := s.app.SaveWorkflow(core.WorkflowDefinition{Name: request.Name, Source: request.Source,
				Destination: request.Destination, Settings: settings})
			return err
		}
		if workflow {
			key, err := core.RandomID()
			if err != nil {
				return err
			}
			_, err = s.app.AddWorkflow(core.WorkflowRequest{
				Name: request.Name, Source: request.Source, Destination: request.Destination,
				IdempotencyKey: key, Settings: &settings, NotBefore: request.NotBefore, ProofPath: proof,
			})
			return err
		}
		_, err := s.app.AddJob(request)
		return err
	}, func() { s.showJobForm = false })
}

func (s *nativeUI) beginJobForm(job *core.Job) {
	s.jobPreview, s.previewOnly = core.Preview{}, false
	s.showJobForm, s.showJobAdvanced, s.jobWorkflow = true, false, false
	s.saveDefinition = false
	s.editingJobID, s.jobProof = "", ""
	if job == nil {
		d := s.app.Snapshot().Defaults
		s.jobName, s.jobSource, s.jobDestination, s.jobIncludes, s.jobSchedule = "", "", "", "", ""
		s.jobOperation, s.jobOperationChoice = "copy", "复制"
		s.jobTransfers, s.jobCheckers, s.jobCPU = itoa(d.Transfers), itoa(d.Checkers), itoa(d.CPUParallelism)
		s.jobChunk, s.jobBuffer, s.jobBandwidth = d.ChunkSize, d.BufferSize, d.Bandwidth
		s.jobChecksum, s.jobCreateDirs, s.jobAllowDelete = false, false, false
		return
	}
	s.editingJobID, s.showJobAdvanced = job.ID, true
	s.jobName, s.jobSource, s.jobDestination = job.Name, job.Source, job.Destination
	s.jobOperation, s.jobOperationChoice = job.Operation, operationLabel(job.Operation)
	s.jobTransfers, s.jobCheckers, s.jobCPU = itoa(job.Transfers), itoa(job.Checkers), itoa(job.CPUParallelism)
	s.jobChunk, s.jobBuffer, s.jobBandwidth = job.ChunkSize, job.BufferSize, job.Bandwidth
	s.jobIncludes = strings.Join(job.Includes, "\n")
	s.jobChecksum, s.jobCreateDirs, s.jobAllowDelete = job.Checksum, job.CreateDirs, job.AllowDelete
	s.jobSchedule = ""
	if job.NotBefore != nil {
		s.jobSchedule = job.NotBefore.Local().Format("2006-01-02 15:04")
	}
}

func (s *nativeUI) loadLog(id string) {
	s.logJobID = id
	s.logText = "加载中…"
	s.invalidate()
	go func() {
		text, err := s.app.JobLog(id, 128*1024)
		s.win.Update(func() {
			if s.logJobID != id {
				return
			}
			if err != nil {
				s.logText = err.Error()
			} else {
				s.logText = core.ValueOr(text, "（暂无日志）")
			}
		})
	}()
}

func (s *nativeUI) invalidate() {
	if s.win != nil {
		s.win.Invalidate()
	}
}

func (s *nativeUI) sparkline(c *ui.Context, samples []float64, width, height float32) {
	if len(samples) < 2 {
		ui.Text(c, "采集吞吐…").FontSize(10).TextColor(c.Theme().TextMuted)
		return
	}
	accent := c.Theme().Accent
	ui.Box(c).Width(width).Height(height).Radius(7).Draw(func(p *ui.Painter, r ui.Rect) {
		maxValue := samples[0]
		for _, value := range samples[1:] {
			if value > maxValue {
				maxValue = value
			}
		}
		if maxValue <= 0 {
			return
		}
		points := make([][2]float32, 0, len(samples))
		for i, value := range samples {
			x := r.X + r.W*float32(i)/float32(max(1, len(samples)-1))
			y := r.Y + r.H - r.H*float32(value/maxValue)
			points = append(points, [2]float32{x, y})
		}
		p.StrokePolyline(points, 1.5, accent)
	}).Label("吞吐速度趋势")
}

func formatDuration(seconds float64) string {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return "—"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", int(seconds))
	}
	minutes := int(seconds / 60)
	if minutes < 1 {
		return "0 秒"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	hours := minutes / 60
	minutes %= 60
	if hours >= 24 {
		return fmt.Sprintf("%d 天 %d 小时", hours/24, hours%24)
	}
	return fmt.Sprintf("%d 小时 %d 分", hours, minutes)
}

func statusColor(c *ui.Context, status string) ui.Color {
	t := c.Theme()
	switch status {
	case "completed":
		return t.Success
	case "failed", "interrupted":
		return t.Danger
	case "running":
		return t.Warning
	default:
		return t.TextMuted
	}
}

func statusLabel(status string) string {
	switch status {
	case "paused":
		return "已暂停"
	case "pending":
		return "等待中"
	case "running":
		return "运行中"
	case "completed":
		return "已完成"
	case "failed":
		return "失败"
	case "cancelled":
		return "已停止"
	case "interrupted":
		return "已中断"
	default:
		return status
	}
}
