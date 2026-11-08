package desktop

import (
	"strings"
	"testing"
	"unicode"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestTranslatedSelectionKeepsCanonicalValuesAcrossLanguages(t *testing.T) {
	selected := "浅色"
	tester := ui.NewTester(func(c *ui.Context) {
		translatedSelect(c, &selected, []string{"浅色", "深色", "跟随系统"}).Label("Theme")
	}, 360, 180)
	tester.SetLocale("en")
	if err := tester.Click("Theme"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if selected != "深色" {
		t.Fatalf("Translated selection changed its canonical value: %q", selected)
	}
	tester.SetLocale("zh-Hans")
	if !tester.HasText("深色") || selected != "深色" {
		t.Fatal("Changing language changed the selected theme")
	}
}

func TestLanguagePickerDistinguishesSystemFromMatchingManualLanguage(t *testing.T) {
	for _, initial := range []string{"", "en"} {
		preference := initial
		applied := "not-called"
		tester := ui.NewTester(func(c *ui.Context) {
			languagePicker(c, &preference, func(tag string) { applied = tag })
		}, 360, 180)
		tester.SetLocale("en")
		expected := "English"
		if initial == "" {
			expected = "Follow system"
		}
		if !tester.HasText(expected) {
			t.Fatalf("%q language mode does not show %q: %v", initial, expected, tester.Texts())
		}
		if initial != "" {
			if err := tester.Click("Language"); err != nil {
				t.Fatal(err)
			}
			if err := tester.Click("Follow system"); err != nil {
				t.Fatal(err)
			}
			if preference != "" || applied != "" {
				t.Fatalf("System did not clear the manual override: %q / %q", preference, applied)
			}
		}
		if err := tester.Click("Language"); err != nil {
			t.Fatal(err)
		}
		if err := tester.Click("English"); err != nil {
			t.Fatal(err)
		}
		if preference != "en" || applied != "en" {
			t.Fatalf("manual selection did not choose its canonical tag: %q / %q", preference, applied)
		}
	}
}

func TestGeneralPreferencesFitMinimumWindowInBothLanguages(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		model.page = "settings"
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		for _, source := range []string{"外观", "语言", "登录时启动"} {
			label := locales.TextIn(language, source)
			bounds, found := tester.Find(label)
			if !found || bounds.X < 208 || bounds.X+bounds.W > 860 || bounds.Y+bounds.H > 560 {
				t.Fatalf("%s general preference %s exceeds minimum viewport: %+v", language, label, bounds)
			}
		}
		if !tester.HasText(locales.In(language).CurrentLanguage(map[string]string{"en": "English", "zh-Hans": "简体中文"}[language])) {
			t.Fatalf("%s system-selected effective language is not visible", language)
		}
	}
}

func TestIndependentProfileShowsUnavailableLoginStartup(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		model.page = "settings"
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if !tester.HasText(locales.In(language).LoginUnavailableForProfile()) {
			t.Fatalf("%s independent profile does not explain unavailable startup", language)
		}
		label := locales.TextIn(language, "登录时启动")
		if control, found := tester.Access(label); !found || !control.Disabled {
			t.Fatal("independent profile's startup control was not disabled")
		}
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		if model.openAtLogin {
			t.Fatal("disabled startup checkbox changed its state")
		}
	}
}

func TestEnglishPagesAndFormsUseOneLanguage(t *testing.T) {
	for _, page := range []string{"overview", "queue", "workflows", "automations", "mounts", "remotes", "resources", "settings"} {
		model := New(testApp(t), "")
		model.page = page
		model.showEngineSettings, model.showDefaults = true, true
		tester := ui.NewTester(model.view, 900, 740)
		tester.SetLocale("en")
		assertNoChineseInterfaceText(t, page, tester.Texts())
	}
	for _, form := range []string{"task", "workflow", "automation", "offline"} {
		model := New(testApp(t), "")
		switch form {
		case "task":
			model.beginJobForm(nil)
			model.showJobAdvanced = true
		case "workflow":
			model.beginWorkflowEditor(core.WorkflowDefinition{}, false)
		case "automation":
			model.beginAutomation(core.Automation{})
		case "offline":
			model.cacheUI.offlineOpen = true
		}
		tester := ui.NewTester(model.view, 900, 740)
		tester.SetLocale("en")
		assertNoChineseInterfaceText(t, form, tester.Texts())
	}
}

func assertNoChineseInterfaceText(t *testing.T, context string, values []string) {
	t.Helper()
	for _, value := range values {
		for _, character := range value {
			if unicode.Is(unicode.Han, character) {
				t.Fatalf("%s retained untranslated interface text: %q", context, value)
			}
		}
	}
}

func TestEnglishViewPreservesUserNamesPathsAndLogs(t *testing.T) {
	model := New(testApp(t), "")
	model.page, model.logJobID = "queue", "synthetic"
	model.logText = "校验"
	model.jobs = []core.Job{{
		ID: "synthetic", Name: "任务队列", Source: "/synthetic/总览",
		Destination: "example:目录/完成", Status: "paused",
	}}
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("en")
	for _, unchanged := range []string{"任务队列", "/synthetic/总览", "example:目录/完成", "校验"} {
		if !tester.HasText(unchanged) {
			t.Fatalf("A user-supplied value was changed: %q", unchanged)
		}
	}
	if !tester.HasText("Task details") || !tester.HasText("Paused") {
		t.Fatal("Interface labels did not use English")
	}
}

func TestCommandSearchAcceptsEnglishAndChinese(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, query := range []string{"QUEUE", "任务队列"} {
			if !commandMatches("任务队列", query, language) {
				t.Fatalf("%s command search did not match %q", language, query)
			}
		}
		if commandMatches("工作流", "queue", language) {
			t.Fatal("Command search matched an unrelated command")
		}
	}
}

func TestEnglishVerificationChoiceRetainsWorkflowSemantics(t *testing.T) {
	model := New(testApp(t), "")
	model.beginWorkflowEditor(core.WorkflowDefinition{
		Name: "Synthetic", Source: "/synthetic/source", Destination: "example:destination",
	}, false)
	tester := ui.NewTester(model.view, 900, 740)
	tester.SetLocale("en")
	if err := tester.Click("Workflow final verification"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Content comparison"); err != nil {
		t.Fatal(err)
	}
	definition, err := editedWorkflow(model.workflowUI)
	if err != nil {
		t.Fatal(err)
	}
	if definition.VerificationMode != "content" || model.workflowUI.verification != "内容比较" {
		t.Fatal("English selection changed the saved verification mode")
	}
	if !strings.Contains(strings.Join(tester.Texts(), "\n"), "may generate download traffic") {
		t.Fatal("Content comparison explanation did not follow the choice")
	}
}

func TestTranslatedWorkflowActionFitsMinimumWindow(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		model.beginWorkflowEditor(core.WorkflowDefinition{}, false)
		model.workflowUI.advanced = true
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		bounds, found := tester.Find(locales.In(language).SaveWorkflow())
		if !found || bounds.X < 0 || bounds.X+bounds.W > 900 || bounds.Y < 0 || bounds.Y+bounds.H > 560 {
			t.Fatalf("%s workflow action exceeded the minimum window: %+v", language, bounds)
		}
	}
}

func TestDriveChunkControlsFollowDestinationCapabilities(t *testing.T) {
	for _, destination := range []string{"/synthetic/local", "unknown:archive", "dropbox:archive", "drive:archive"} {
		for _, form := range []string{"task", "workflow"} {
			model := New(testApp(t), "")
			model.remotes = []core.RemoteInfo{{Name: "drive", Type: "drive"}, {Name: "dropbox", Type: "dropbox"}}
			if form == "task" {
				model.beginJobForm(nil)
				model.jobDestination, model.showJobAdvanced = destination, true
			} else {
				model.beginWorkflowEditor(core.WorkflowDefinition{Destination: destination}, false)
				model.workflowUI.advanced = true
			}
			tester := ui.NewTester(model.view, 900, 740)
			tester.SetLocale("en")
			_, exists := tester.Find("Google Drive upload chunk")
			if exists != (destination == "drive:archive") {
				t.Fatalf("%s/%s displayed an inapplicable Drive chunk setting", form, destination)
			}
		}
	}
}
