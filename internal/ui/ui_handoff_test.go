package desktop

import (
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestHandoffDialogKeepsPreviewAndBeginVisibleInBothLanguages(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		model.handoffUI.open = true
		model.handoffUI.request.StatePath = "/synthetic/state.json"
		model.handoffUI.request.ConfigPath = "/synthetic/rclone.conf"
		model.handoffUI.preview = core.HandoffPreview{PreviewKey: "synthetic", Jobs: []core.Job{{ID: "sample", Name: "Sample upload", Status: "running"}}}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		for _, source := range []string{"预览导入", "开始交接"} {
			bounds, ok := tester.Find(locales.TextIn(language, source))
			if !ok || bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.W > 900 || bounds.Y+bounds.H > 560 {
				t.Fatalf("handoff action outside minimum window: %s %+v", source, bounds)
			}
		}
		if language == "en" {
			assertNoChineseInterfaceText(t, "handoff form", tester.Texts())
		}
	}
}

func TestHandoffTaskHasNoLocalStopControl(t *testing.T) {
	model := New(testApp(t), "")
	model.page = "queue"
	model.jobs = []core.Job{{ID: "legacy", Name: "Imported upload", Status: "handoff", Source: "storage:source", Destination: "storage:target"}}
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("en")
	if !tester.HasText("Previous client is running") {
		t.Fatal("missing task ownership")
	}
	if _, ok := tester.Access("Pause"); ok {
		t.Fatal("external step exposed a local stop control")
	}
}

func TestHandoffExplainsOwnershipWithoutDuplicateRuntimeProse(t *testing.T) {
	model := New(testApp(t), "")
	model.page = "queue"
	model.handoff = &core.LegacyHandoff{Status: "waiting"}
	model.queueBlockReason = "Imported tasks are waiting for the previous client to finish its active step"
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("zh-Hans")
	if tester.HasText(model.queueBlockReason) {
		t.Fatal("queue repeated an untranslated handoff diagnostic")
	}
	model.handoff.Status = "blocked"
	model.handoff.Message = "The previous queue advanced to another step; return ownership to the previous client"
	tester = ui.NewTester(model.view, 900, 740)
	tester.SetLocale("zh-Hans")
	if !tester.HasText("旧队列已执行其他步骤，请交还原客户端处理") {
		t.Fatal("handoff diagnostic did not follow UI language")
	}
}
