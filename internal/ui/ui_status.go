package desktop

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) statusBar(c *ui.Context) {
	t := c.Theme()
	waiting, failed := 0, 0
	for _, job := range s.jobs {
		if job.Status == "pending" {
			waiting++
		}
		if job.Status == "failed" || job.Status == "interrupted" {
			failed++
		}
	}
	ui.Row(c).Height(28).Shrink(0).Padding(0, 24, 0, 28).Gap(9).Children(func() {
		label := "引擎就绪"
		color := t.Success
		if s.version == "" {
			label = "引擎未就绪"
			color = t.Warning
		}
		ui.Box(c).Size(4, 4).Radius(2).Background(color)
		ui.Text(c, label).FontSize(10).TextColor(t.TextMuted).Tooltip(s.version)
		if active := s.activeJob(); active != nil {
			ui.Text(c, fmt.Sprintf("· %.2f MiB/s", active.Speed/1048576)).FontSize(10).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if failed > 0 {
			ui.Text(c, fmt.Sprintf("%d 项需要处理", failed)).FontSize(10).TextColor(t.Warning)
		}
		label = fmt.Sprintf("%d 项等待", waiting)
		if s.paused {
			label = "队列已暂停 · " + label
		}
		ui.Text(c, label).FontSize(10).TextColor(t.TextMuted)
	})
}
