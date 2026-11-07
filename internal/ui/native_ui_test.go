package desktop

import (
	"github.com/DeliciousBuding/rclonego/internal/core"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeNavigationAndKeyActions(t *testing.T) {
	app := testApp(t)
	model := New(app, "127.0.0.1:8797")
	tester := ui.NewTester(model.view, 1180, 760)
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
}

func TestWindowMaterialAndCaptionClearance(t *testing.T) {
	model := New(testApp(t), "127.0.0.1:8797")
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetVibrancy(true)
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
}

func TestActivityStatusDistinguishesConnectionFromActivity(t *testing.T) {
	model := New(testApp(t), "127.0.0.1:8797")
	model.activity = core.ActivityStatus{Available: true, Stats: &core.RCStats{}}
	tester := ui.NewTester(model.view, 1120, 740)
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
