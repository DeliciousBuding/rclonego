package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

func TestActiveTaskUsesVerificationProgressAndUnknownTotals(t *testing.T) {
	for _, scenario := range []struct {
		job      core.Job
		phase    string
		progress float64
	}{
		{core.Job{Operation: "check", ChecksDone: 3, ChecksTotal: 10, Bytes: 900, TotalBytes: 1000}, "Verifying", .3},
		{core.Job{Operation: "copy", CheckingFiles: []string{"sample.txt"}, ChecksDone: 2, ChecksTotal: 5}, "Comparing", .4},
		{core.Job{Operation: "copy", CheckingFiles: []string{"sample.txt"}, Speed: 10, Bytes: 200, TotalBytes: 1000}, "Transferring", .2},
		{core.Job{Operation: "check"}, "Verifying", -1},
		{core.Job{Operation: "copy"}, "Preparing", -1},
	} {
		model := New(testApp(t), "")
		model.page = "queue"
		scenario.job.ID, scenario.job.Name, scenario.job.Status = "synthetic", "Example task", "running"
		scenario.job.Source, scenario.job.Destination = "/synthetic/source", "example:archive"
		model.jobs = []core.Job{scenario.job}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale("en")
		if !tester.HasText(scenario.phase) || jobProgress(scenario.job) != scenario.progress {
			t.Fatalf("%s shows the wrong phase or measure: %v", scenario.phase, tester.Texts())
		}
		if scenario.phase != "Transferring" && tester.HasText("Speed") {
			t.Fatal("An inactive byte-transfer metric displaced verification or preparation")
		}
		if scenario.progress < 0 && strings.Contains(strings.Join(tester.Texts(), "\n"), "0 / 0") {
			t.Fatal("An unknown file total was presented as a completed zero-file count")
		}
		if len(scenario.job.CheckingFiles) > 0 {
			*model.openState("synthetic/checks", true) = false
			model.pruneTreeState()
			tester.Frame()
			tester.Advance(500 * time.Millisecond)
			if tester.HasText("sample.txt") {
				t.Fatal("Refresh reopened the collapsed comparison file list")
			}
		}
		for _, label := range []string{"Pause", "View active task details", "More active task actions"} {
			bounds, found := tester.Find(label)
			if !found || bounds.X < 208 || bounds.X+bounds.W > 860 || bounds.Y+bounds.H > 560 {
				t.Fatalf("%s action exceeds the minimum canvas: %s %+v", scenario.phase, label, bounds)
			}
		}
	}
}

func TestErrorCanBeReadAndDismissedWithoutBlockingNavigation(t *testing.T) {
	model := New(testApp(t), "")
	model.page, model.isError, model.notice = "queue", true, strings.Repeat("Example connection error. ", 15)
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if bounds, found := tester.Find(model.notice); !found || bounds.H < 30 || bounds.X+bounds.W > 860 {
		t.Fatalf("Error message is clipped to a single line or outside the canvas: %+v", bounds)
	}
	if err := tester.Click("Dismiss error"); err != nil {
		t.Fatal(err)
	}
	if model.isError || model.notice != "" {
		t.Fatal("The dismissed message still obscures the page")
	}
	if err := tester.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if model.page != "settings" {
		t.Fatal("Dismissing the error broke navigation")
	}
}

func TestMountMoreMenuPreservesIdentityAndProtectsRunningConfiguration(t *testing.T) {
	model := New(testApp(t), "")
	model.page = "mounts"
	mount := core.MountView{MountConfig: core.MountConfig{
		ID: "synthetic", Name: strings.Repeat("Example mount ", 12),
		Remote: "example:archive", Drive: "/synthetic/mount",
	}}
	model.mounts = []core.MountView{mount}
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if !tester.HasText("example:archive → /synthetic/mount") {
		t.Fatal("Mount route is unavailable in the list")
	}
	if err := tester.Click("More mount actions"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Save offline copy"); err != nil {
		t.Fatal(err)
	}
	if !model.cacheUI.offlineOpen || model.cacheUI.source != mount.Remote || len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("Opening an offline-copy form lost its source or started a task")
	}
	tester.Key(0, ui.KeyEscape)
	model.mounts[0].Running = true
	tester.Frame()
	if err := tester.Click("More mount actions"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Remove this mount configuration"); err == nil {
		t.Fatal("A running mount offered configuration deletion")
	}
}

func TestHistoryFiltersAndPagingAreShownWhenUsable(t *testing.T) {
	model := New(testApp(t), "")
	model.page = "workflows"
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("en")
	if _, found := tester.Find("Filter runs on this page"); found {
		t.Fatal("Empty history displayed a useless filter")
	}
	if _, found := tester.Find("Next page"); found {
		t.Fatal("Empty history displayed pagination")
	}
	model.runs = []core.Run{{ID: "one", Name: "Example run", CreatedAt: time.Now()}}
	tester.Frame()
	if _, found := tester.Find("Filter runs on this page"); !found {
		t.Fatal("Existing history cannot be filtered")
	}
	if _, found := tester.Find("Next page"); found {
		t.Fatal("One page of history displayed unavailable pagination")
	}
}
