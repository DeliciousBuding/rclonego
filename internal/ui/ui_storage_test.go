package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestStorageDetailsKeepUnknownCapacityAndActionsVisible(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		model.page, model.storageDetail, model.showStorageDetails = "remotes", "storage", true
		model.remotes = []core.RemoteInfo{{Name: "storage", Type: "drive"}}
		model.storageUI["storage"] = &storageUIState{
			info:  core.StorageInfo{Name: "storage", UpdatedAt: time.Now()},
			quota: core.RemoteQuota{Name: "storage", Status: "unsupported"},
		}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if language == "en" {
			assertNoChineseInterfaceText(t, "storage details", tester.Texts())
		}
		if !tester.HasText(locales.TextIn(language, "服务未提供账户容量")) {
			t.Fatal("missing quota appeared as a known total")
		}
		if strings.Contains(strings.Join(tester.Texts(), "\n"), "0 B") {
			t.Fatal("unknown quota appeared as zero bytes")
		}
		bounds, ok := tester.Find(locales.TextIn(language, "浏览文件"))
		if !ok || bounds.X+bounds.W > 900 || bounds.Y+bounds.H > 560 {
			t.Fatalf("storage action is outside the minimum window: %v", bounds)
		}
	}
}

func TestDirectoryBrowserStartsAnUnsubmittedTransfer(t *testing.T) {
	model := New(testApp(t), "")
	model.browserUI = browserUIState{open: true, path: "storage:folder", result: core.BrowseResult{Path: "storage:folder"}}
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("en")
	if err := tester.Click("New transfer…"); err != nil {
		t.Fatal(err)
	}
	if !model.showJobForm || model.jobSource != "storage:folder" || model.jobDestination != "" || model.browserUI.open {
		t.Fatal("directory browser did not prepare the existing task form")
	}
	if len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("browsing started a transfer without task submission")
	}
}

func TestQuotaIncludesOtherServicesWithoutInventingMissingTotals(t *testing.T) {
	total, free, used, other := int64(100), int64(60), int64(30), int64(10)
	quota := core.RemoteQuota{Total: &total, Free: &free, Used: &used, Other: &other}
	if got := quotaOccupied(quota); got == nil || *got != 40 {
		t.Fatal("account occupancy excluded usage from other services")
	}
	if got := quotaOccupied(core.RemoteQuota{Used: &used}); got == nil || *got != 30 {
		t.Fatal("provider usage was lost without total and free")
	}
	if got := quotaOccupied(core.RemoteQuota{}); got != nil {
		t.Fatal("unknown occupancy became zero")
	}
}
