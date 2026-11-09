package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type nativeUI struct {
	app         *core.App
	win         *mygo.Window
	page        string
	api         string
	refreshMu   sync.Mutex
	pollVisible atomic.Bool
	pollActive  atomic.Bool
	pollWake    chan struct{}

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
	languageOverride   string
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
	showEngineSettings  bool
	showResourceDetails bool
	externalOpen        bool
	showJobDetails      bool
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
	previewList         ui.ListState
	jobPreviewKey       string
	jobMaxDelete        string
	previewOnly         bool
	jobAttempted        bool
	showAbout           bool
	showLicenses        bool
	licenseChoice       string
	commandOpen         bool
	commandQuery        string
	commandIndex        int
	workflowUI          workflowUIState
	automationUI        automationUIState
	automations         []core.Automation
	browserUI           browserUIState
	filesUI             fileWorkspaceState
	fileOperationFocus  bool
	health              core.ServiceHealth
	cacheUI             cacheUIState
	storageUI           map[string]*storageUIState
	storageDetail       string
	showStorageDetails  bool
	handoffUI           handoffUIState
	handoff             *core.LegacyHandoff
}

func New(app *core.App, api string) *nativeUI {
	state := app.Snapshot()
	defaults := state.Defaults
	model := &nativeUI{
		app: app, page: "overview", api: api, newRemoteType: "drive", appearance: state.Appearance,
		appearanceChoice: appearanceLabel(state.Appearance),
		languageOverride: savedLanguagePreference(app.DataDirectory()),
		jobOperation:     "copy", jobOperationChoice: "复制", remoteTypeChoice: "Google Drive",
		jobTransfers: itoa(defaults.Transfers), jobCheckers: itoa(defaults.Checkers), jobChunk: defaults.ChunkSize, jobBuffer: defaults.BufferSize,
		jobCPU: itoa(defaults.CPUParallelism), defaults: defaults,
		defaultTransfers: itoa(defaults.Transfers), defaultCheckers: itoa(defaults.Checkers),
		defaultChunk: defaults.ChunkSize, defaultBuffer: defaults.BufferSize, defaultBandwidth: defaults.Bandwidth,
		defaultCPU:  itoa(defaults.CPUParallelism),
		jobSelected: -1, jobTable: ui.ListState{}, jobSpeeds: map[string][]float64{},
		treeOpen: map[string]*bool{}, waitingOpen: true,
		storageUI:    map[string]*storageUIState{},
		externalOpen: true,
		pollWake:     make(chan struct{}, 1),
	}
	model.pollVisible.Store(true)
	return model
}

func (s *nativeUI) Attach(win *mygo.Window) {
	s.win = win
	s.setVisible(win.IsVisible() && !win.IsMinimized())
	win.OnShow(func() { s.setVisible(true) })
	win.OnHide(func() { s.setVisible(false) })
	win.OnMinimize(func() { s.setVisible(false) })
	win.OnRestore(func() { s.setVisible(true) })
}

func (s *nativeUI) shouldPoll(tick int) bool {
	return s.pollVisible.Load() && (s.pollActive.Load() || tick%5 == 0)
}

func (s *nativeUI) setVisible(visible bool) {
	s.pollVisible.Store(visible)
	if visible {
		select {
		case s.pollWake <- struct{}{}:
		default:
		}
	}
}

func (s *nativeUI) StartPolling(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		tick := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.pollWake:
				s.refresh(ctx, true)
			case <-ticker.C:
				tick++
				if !s.shouldPoll(tick) {
					continue
				}
				s.refresh(ctx, tick%4 == 0)
			}
		}
	}()
}

func (s *nativeUI) refresh(ctx context.Context, includeRemoteState bool) {
	if ctx.Err() != nil || !s.refreshMu.TryLock() {
		return
	}
	defer s.refreshMu.Unlock()
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
	activity := core.CurrentActivity(state, runtime)
	active := activity.Available && activity.Stats != nil && (activity.Stats.Speed > 0 || len(activity.Stats.Transferring) > 0 || len(activity.Stats.Checking) > 0)
	for _, job := range state.Jobs {
		active = active || job.Status == "running" || job.Status == "handoff"
	}
	s.pollActive.Store(active)
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
			updateNativeFrame(s.win)
		}
		s.remotes, s.mounts, s.version = runtime.Remotes, runtime.Mounts, runtime.Version
		for name := range s.storageUI {
			found := false
			for _, remote := range s.remotes {
				found = found || remote.Name == name
			}
			if !found {
				delete(s.storageUI, name)
			}
		}
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
		s.refreshCompletedFiles(jobs)
		s.handoff = state.Handoff
		s.workflows, s.runs = state.Workflows, state.Runs
		s.automations = s.app.ListAutomations()
		s.external, s.resources = runtime.External, runtime.Resources
		s.queueBlockReason = runtime.QueueBlockReason
		s.health = runtime.Health
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
				s.isError, s.notice = false, label+locales.CompletedSuffix()
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
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		pageHeading(c, "盘符映射", locales.In(c.Locale()).MountCount(len(s.mounts)), func() {
			actionButton(c.Key("new-mount"), iconPlus, "添加映射…", true).Disabled(s.busy).OnClick(func() {
				s.beginMountEdit(core.MountConfig{})
			})
		})
		s.mountDependencyHint(c)
		contentScroll(c).Grow(1).Gap(0).Children(func() {
			if len(s.mounts) == 0 {
				emptyState(c, iconMounts, "暂无映射", "添加映射，将云端目录作为本机磁盘使用")
			}
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
			label := mount.Drive
			if !strings.HasSuffix(label, ":") {
				label = text(c, "目录")
			}
			ui.Text(c, label).FontSize(14).Bold().TextColor(t.Accent).Tooltip(mount.Drive)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(7).MinWidth(0).Children(func() {
				s.storageIcon(c, mount.Remote, iconRemotes, 18)
				ui.Text(c, mount.Name).Grow(1).MinWidth(0).Bold().SingleLine().Tooltip(mount.Name)
			})
			pathSummary(c, mount.Remote, mount.Drive)
			if mount.External {
				ui.Text(c, text(c, "由其他程序管理")).FontSize(11).TextColor(t.TextMuted)
			}
		})
		iconButton(c.Key("browse-"+mount.ID), iconFolder, "浏览文件").OnClick(func() { s.openFiles(mountBrowsePath(mount)) })
		if mount.Running {
			statusPill(c, "运行中", t.Success)
			ui.Button(c.Key("stop-"+mount.ID), text(c, "卸载")).Padding(6, 10).Tooltip(text(c, "卸载此盘符")).OnClick(func() {
				cfg := mount.MountConfig
				s.run(text(c, "卸载 ")+cfg.Drive, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					return s.app.StopMount(ctx, cfg)
				})
			}).Disabled(s.busy || !s.app.OwnsMount(mount.ID))
		} else {
			statusDot(c, "未挂载", t.TextMuted)
			ui.Button(c.Key("start-"+mount.ID), text(c, "挂载")).Padding(6, 10).Tooltip(text(c, "挂载此盘符")).OnClick(func() {
				cfg := mount.MountConfig
				s.run(text(c, "挂载 ")+cfg.Drive, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					_, err := s.app.StartMount(ctx, cfg)
					return err
				})
			}).Disabled(s.busy)
		}
		iconButton(c.Key("more-"+mount.ID), iconMore, "更多映射操作").Disabled(s.busy).Menu(func(m *ui.Menu) {
			if m.Item(text(c, "浏览文件")).Chosen() {
				s.openFiles(mountBrowsePath(mount))
			}
			if m.Item(text(c, "在文件管理器中打开")).Disabled(!mount.DriveAvailable).Chosen() {
				s.openMountedPath(mount)
			}
			if m.Item(text(c, "编辑…")).Disabled(mount.External).Chosen() {
				s.beginMountEdit(mount.MountConfig)
			}
			if m.Item(text(c, "查看缓存状态")).Disabled(!mount.Running || !s.app.OwnsMount(mount.ID)).Chosen() {
				s.inspectCache(mount.ID)
			}
			if m.Item(text(c, "保存离线副本")).Chosen() {
				s.cacheUI.offlineOpen, s.cacheUI.source, s.cacheUI.name = true, mount.Remote, mount.Name+locales.OfflineCopySuffix()
			}
			m.Separator()
			if m.Item(text(c, "删除此映射配置")).Disabled(mount.Running || mount.External).Chosen() {
				s.run(text(c, "删除映射"), func() error {
					state := s.app.Snapshot()
					mounts := make([]core.MountConfig, 0, len(state.Mounts))
					for _, cfg := range state.Mounts {
						if cfg.ID != mount.ID {
							mounts = append(mounts, cfg)
						}
					}
					return s.app.SaveMounts(mounts)
				})
			}
		})
	})
}

func (s *nativeUI) beginMountEdit(cfg core.MountConfig) {
	s.editingMount = true
	s.mountID, s.mountName, s.mountRemote = cfg.ID, cfg.Name, cfg.Remote
	s.mountDrive, s.mountVolume, s.mountCache = cfg.Drive, cfg.VolumeName, cfg.CacheDir
	if cfg.ID == "" {
		targets := s.app.MountTargets()
		s.mountDrive = targets.Suggested
		s.mountCache = s.app.DefaultMountCache()
	}
	s.mountMax, s.mountAge = core.ValueOr(cfg.CacheMax, "15G"), core.ValueOr(cfg.CacheMaxAge, "24h")
	s.invalidate()
}

func (s *nativeUI) addJob() {
	previewOnly := s.previewOnly
	s.previewOnly, s.jobAttempted = false, true
	request, issue := s.editedJob()
	if issue != nil {
		if issue.field == "parameters" || issue.field == "schedule" {
			s.showJobAdvanced = true
		}
		s.invalidate()
		return
	}
	if previewOnly || s.editingJobID == "" && request.Operation == "sync" && s.jobPreview.ID == "" {
		var preview core.Preview
		previewKey := s.currentPreviewKey()
		s.runAfter(locales.Text("预览更改"), func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			var err error
			preview, err = s.app.PreviewJob(ctx, request)
			return err
		}, func() { s.jobPreview, s.jobPreviewKey = preview, previewKey })
		return
	}
	editID, workflow, proof := s.editingJobID, s.jobWorkflow, s.jobProof
	saveDefinition := s.saveDefinition
	s.runAfter(locales.Text("保存任务"), func() error {
		settings := core.TransferSettings{Transfers: request.Transfers, Checkers: request.Checkers, ChunkSize: request.ChunkSize, BufferSize: request.BufferSize, CPUParallelism: *request.CPUParallelism, Bandwidth: request.Bandwidth}
		if settings.Bandwidth == "off" {
			settings.Bandwidth = ""
		}
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
	s.jobMaxDelete, s.jobPreviewKey = "10", ""
	s.showJobForm, s.showJobAdvanced, s.jobWorkflow = true, false, false
	s.jobAttempted, s.isError, s.notice = false, false, ""
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
	if job.MaxDelete > 0 {
		s.jobMaxDelete = itoa(job.MaxDelete)
	}
	s.jobSchedule = ""
	if job.NotBefore != nil {
		s.jobSchedule = job.NotBefore.Local().Format("2006-01-02 15:04")
	}
}

func (s *nativeUI) loadLog(id string) {
	s.logJobID = id
	s.showJobDetails = false
	s.logText = locales.Loading()
	s.invalidate()
	if s.win == nil {
		return
	}
	go func() {
		text, err := s.app.JobLog(id, 128*1024)
		s.win.Update(func() {
			if s.logJobID != id {
				return
			}
			if err != nil {
				s.logText = err.Error()
			} else {
				s.logText = core.ValueOr(text, locales.NoLog())
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
		ui.Text(c, text(c, "采集吞吐…")).FontSize(10).TextColor(c.Theme().TextMuted)
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
	}).Label(text(c, "吞吐速度趋势"))
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
	case "handoff":
		return "旧客户端执行中"
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
