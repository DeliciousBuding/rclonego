package desktop

import (
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/internal/version"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestProviderLogosRenderAsColorVectors(t *testing.T) {
	logos := []struct {
		name string
		svg  *ui.SVG
	}{{"Google", googleGLogo}, {"Drive", googleDriveLogo}, {"OneDrive", oneDriveLogo}, {"Dropbox", dropboxLogo}, {"rclone", rcloneLogo}}
	for _, logo := range logos {
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Center().Children(func() { brandIcon(c, logo.svg, 32).Label(logo.name) })
		}, 64, 64)
		tester.SetLocale("zh-Hans")
		image := tester.Image()
		colored := 0
		for y := 16; y < 48; y++ {
			for x := 16; x < 48; x++ {
				p := image.RGBAAt(x, y)
				if p.A > 0 && (int(p.R)-int(p.G) > 20 || int(p.G)-int(p.R) > 20 || int(p.B)-int(p.R) > 20) {
					colored++
				}
			}
		}
		if colored < 30 {
			t.Fatalf("%s logo did not render enough colored pixels: %d", logo.name, colored)
		}
	}
}

func TestAboutShowsSharedVersionAndProjectLinks(t *testing.T) {
	model := New(testApp(t), "http://127.0.0.1:12345")
	model.page = "settings"
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	if err := tester.Click("关于 RcloneGo"); err != nil {
		t.Fatal(err)
	}
	if !model.showAbout || !tester.HasText(version.Version+" · Apache-2.0") {
		t.Fatal("About version missing")
	}
	for _, label := range []string{"GitHub", "反馈问题", "版本发布"} {
		rect, ok := tester.Find(label)
		if !ok || rect.X < 0 || rect.X+rect.W > 900 {
			t.Fatalf("About link not visible: %s %v", label, rect)
		}
	}
	tester.Key(0, ui.KeyEscape)
	if model.showAbout {
		t.Fatal("About did not close with Escape")
	}
}

func TestNativeNavigationAndKeyActions(t *testing.T) {
	app := testApp(t)
	model := New(app, "127.0.0.1:8797")
	tester := ui.NewTester(model.view, 1180, 760)
	tester.SetLocale("zh-Hans")
	if !tester.HasText("总览") || !tester.HasText("传输总览") {
		t.Fatalf("overview not rendered: %q", tester.Texts())
	}
	if err := tester.Click("任务队列"); err != nil {
		t.Fatal(err)
	}
	if model.page != "queue" || !tester.HasText("新建任务…") {
		t.Fatalf("queue navigation failed: page=%q texts=%q", model.page, tester.Texts())
	}
	if err := tester.Click("新建任务…"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("加入队列") {
		t.Fatal("queue action missing")
	}
	tester.Key(0, ui.KeyEscape)
	if model.showJobForm {
		t.Fatal("Escape did not dismiss the task dialog")
	}
	if err := tester.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if model.page != "settings" || !tester.HasText("外观") || !tester.HasText("语言") || !tester.HasText("登录时启动") || !tester.HasText("客户端更新") {
		t.Fatal("Application preferences are not reachable from Settings")
	}
	if err := tester.Click("运行与资源"); err != nil {
		t.Fatal(err)
	}
	if model.page != "resources" || tester.HasText("语言") || tester.HasText("外观") {
		t.Fatal("Resources still contains application preferences")
	}
	if err := tester.Click("查看引擎状态"); err != nil {
		t.Fatal(err)
	}
	if model.page != "settings" || !model.showEngineSettings || !tester.HasText("下载推荐版本") {
		t.Fatal("Engine status did not open its settings")
	}
}

func TestResourceControlsStayInsideScrollGutterWithLongPaths(t *testing.T) {
	for _, width := range []int{900, 1120, 1440} {
		for _, theme := range []string{"light", "dark"} {
			model := New(testApp(t), "http://127.0.0.1:12345")
			model.page, model.appearance = "resources", theme
			model.enginePath = "/synthetic/" + strings.Repeat("long-directory/", 40) + "rclone"
			model.engineArchive = model.enginePath + ".zip"
			model.resources = core.ResourceSnapshot{LogicalCPUs: 128, Goroutines: 100000,
				Processes: []core.ProcessResource{{PID: 1, Name: strings.Repeat("LongProcessName", 20), Threads: 10000}}}
			tester := ui.NewTester(model.view, width, 740)
			tester.SetLocale("zh-Hans")
			tester.Scroll(float32(width-60), 500, 0, 1)
			counts, ok := tester.Find("运行计数")
			if !ok || counts.X < 208 || counts.X+counts.W > float32(width-12-28-16)+1 {
				t.Fatalf("%d/%s: resource counts exceed content gutter: %v", width, theme, counts)
			}
			model.page, model.showEngineSettings = "settings", true
			tester.Frame()
			for _, label := range []string{"引擎路径", "离线发行包路径", "使用此引擎", "导入发行包"} {
				rect, ok := tester.Find(label)
				if !ok {
					t.Fatalf("%d/%s: missing %s", width, theme, label)
				}
				if rect.X < 208 || rect.X+rect.W > float32(width-12-28-16)+1 {
					t.Fatalf("%d/%s: %s exceeds content gutter: %v", width, theme, label, rect)
				}
			}
			input, _ := tester.Find("引擎路径")
			button, _ := tester.Find("使用此引擎")
			if input.X+input.W > button.X {
				t.Fatalf("%d/%s: input overlaps action", width, theme)
			}
		}
	}
}

func TestNarrowTaskDetailsHaveReadableHeightAndNoSideOverlap(t *testing.T) {
	model := New(testApp(t), "http://127.0.0.1:12345")
	model.page, model.logJobID = "queue", "synthetic-step"
	model.jobs = []core.Job{{ID: model.logJobID, Name: strings.Repeat("Long synthetic name ", 12), Source: strings.Repeat("/source", 20),
		Destination: strings.Repeat("/destination", 20), Status: "paused"}}
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	header, ok := tester.Find("任务详情")
	if !ok {
		t.Fatal("task details missing")
	}
	if header.X < 208 || header.X+header.W > 900-40 {
		t.Fatalf("task details outside content: %v", header)
	}
	name, ok := tester.Find(model.jobs[0].Name)
	if !ok || name.H < 18 {
		t.Fatalf("task name clipped vertically: %v", name)
	}
	log, ok := tester.Find("日志")
	if !ok || log.Y <= name.Y+name.H {
		t.Fatalf("log overlaps detail: %v / %v", log, name)
	}
	target, ok := tester.Find(model.jobs[0].Destination)
	if !ok || target.Y+target.H > 560 || target.Y >= log.Y {
		t.Fatalf("target is not visible before the log at minimum height: %v / %v", target, log)
	}
}

func TestWindowMaterialAndCaptionClearance(t *testing.T) {
	model := New(testApp(t), "127.0.0.1:8797")
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	tester.SetVibrancy(true)
	tester.SetWindowActive(true)
	tester.SetTitleBar(ui.TitleBar{Height: 56, Right: 138})
	// The sidebar must let the native material through. The content must
	// remain opaque for legibility, including above a busy desktop.
	image := tester.Image()
	if pixel := image.RGBAAt(100, 300); pixel.A == 255 {
		t.Fatalf("native material is covered: sidebar pixel=%v", pixel)
	}
	if pixel := image.RGBAAt(500, 500); pixel.A != 255 {
		t.Fatalf("content is not opaque: content pixel=%v", pixel)
	}
	header := image.RGBAAt(211, 50)
	corner := image.RGBAAt(209, 57)
	if header != corner {
		t.Fatalf("canvas corner exposes a different material: header=%v corner=%v", header, corner)
	}
	bounds, ok := tester.Find("刷新状态")
	if !ok || bounds.X+bounds.W > 900-138 {
		t.Fatalf("toolbar overlaps native window controls: bounds=%v", bounds)
	}
	for _, theme := range []string{"light", "dark"} {
		model.appearance = theme
		tester.SetWindowActive(false)
		image = tester.Image()
		sidebar := image.RGBAAt(100, 300)
		header = image.RGBAAt(211, 50)
		if sidebar.A != 255 || header != sidebar {
			t.Fatalf("%s inactive chrome exposes the system gray or different tints: sidebar=%v header=%v", theme, sidebar, header)
		}
		tester.SetWindowActive(true)
		if pixel := tester.Image().RGBAAt(100, 300); pixel.A >= 128 {
			t.Fatalf("%s active tint obscures the native material: %v", theme, pixel)
		}
	}
}

func TestActivityStatusDistinguishesConnectionFromActivity(t *testing.T) {
	model := New(testApp(t), "127.0.0.1:8797")
	model.activity = core.ActivityStatus{Available: true, Stats: &core.RCStats{}}
	tester := ui.NewTester(model.view, 1120, 740)
	tester.SetLocale("zh-Hans")
	_, uploading := tester.AccessByRole(ui.RoleText, "正在上传")
	_, waiting := tester.AccessByRole(ui.RoleText, "等待传输")
	if uploading || !waiting {
		t.Fatal("connected but idle RC was displayed as uploading")
	}
	model.activity.Stats.Transferring = []core.RCTransfer{{Name: "sample.bin", SrcFs: "//?/X:/sample/part-a", DstFs: "example-remote{test}:archive/part-a"}}
	tester.Frame()
	if !tester.HasText("正在上传") || model.activityPhase() != `X:\sample\part-a  →  example-remote:archive/part-a` {
		t.Fatalf("active phase not shown correctly: %s", model.activityPhase())
	}
}

func TestWorkflowProgressUsesStableAggregate(t *testing.T) {
	active := core.Job{ID: "b", WorkflowID: "wf", WorkflowTotalBytes: 100, WorkflowInitialBytes: 20, Bytes: 15, Status: "running"}
	jobs := []core.Job{
		{ID: "a", WorkflowID: "wf", Status: "completed", Bytes: 10},
		active,
		{ID: "c", WorkflowID: "wf", Status: "pending"},
	}
	done, total, ok := workflowProgress(jobs, &active)
	if !ok || done != 45 || total != 100 {
		t.Fatalf("unexpected aggregate: %d/%d %v", done, total, ok)
	}
	active.Bytes = 90
	done, _, _ = workflowProgress(jobs, &active)
	if done != 100 {
		t.Fatalf("aggregate exceeded total: %d", done)
	}
}
