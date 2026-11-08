package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBackgroundPollingStopsUntilShown(t *testing.T) {
	model := New(testApp(t), "")
	model.setVisible(false)
	model.pollActive.Store(true)
	for tick := 1; tick <= 20; tick++ {
		if model.shouldPoll(tick) {
			t.Fatal("hidden window scheduled an expensive refresh")
		}
	}
	model.setVisible(true)
	select {
	case <-model.pollWake:
	default:
		t.Fatal("showing window did not request immediate refresh")
	}
	model.pollActive.Store(false)
	if model.shouldPoll(1) || !model.shouldPoll(5) {
		t.Fatal("idle window did not reduce refresh frequency")
	}
	model.pollActive.Store(true)
	if !model.shouldPoll(1) {
		t.Fatal("active transfer did not retain responsive updates")
	}
}

func TestCommandSearchNavigatesAndEscapeCloses(t *testing.T) {
	model := New(testApp(t), "")
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	tester.Key(ui.Cmd, ui.KeyK)
	if !model.commandOpen {
		t.Fatal("command keyboard shortcut did not open palette")
	}
	model.commandQuery = "任务队列"
	tester.Frame()
	tester.Key(0, ui.KeyEnter)
	if model.commandOpen || model.page != "queue" {
		t.Fatal("command search did not navigate to queue")
	}
	tester.Key(ui.Cmd, ui.KeyK)
	tester.Key(0, ui.KeyEscape)
	if model.commandOpen {
		t.Fatal("Escape did not dismiss command palette")
	}
}
