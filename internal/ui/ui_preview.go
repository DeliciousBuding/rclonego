package desktop

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) currentPreviewKey() string {
	return strings.Join([]string{s.jobOperation, s.jobSource, s.jobDestination, s.jobIncludes, s.jobTransfers, s.jobCheckers, s.jobBandwidth,
		s.jobChunk, s.jobBuffer, s.jobCPU, s.jobSchedule, s.jobMaxDelete, fmt.Sprint(s.jobChecksum), fmt.Sprint(s.jobCreateDirs)}, "\x00")
}

func (s *nativeUI) previewPanel(c *ui.Context) {
	p := s.jobPreview
	t := c.Theme()
	ui.Column(c).Gap(9).Padding(12).Radius(8).Background(t.Surface).Children(func() {
		sectionHeading(c, "更改预览")
		ui.Row(c).Gap(16).Wrap().Children(func() {
			s.metric(c, "新增", fmt.Sprint(p.Summary.Added))
			s.metric(c, "修改", fmt.Sprint(p.Summary.Changed))
			s.metric(c, "删除", fmt.Sprint(p.Summary.Deleted))
		})
		ui.Text(c, p.Source+" → "+p.Destination).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(p.Source + " → " + p.Destination)
		ui.List(c.Key("preview-files"), &s.previewList, len(p.Entries), func(i int) {
			entry := p.Entries[i]
			label, color := "修改", t.Accent
			switch entry.Action {
			case "added":
				label = "新增"
			case "deleted":
				label, color = "删除", t.Danger
			case "unchanged":
				label, color = "相同", t.TextMuted
			case "destination_only":
				label, color = "仅在目标", t.TextMuted
			}
			ui.Row(c).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, text(c, label)).Width(78).FontSize(11).TextColor(color).SingleLine()
				ui.Text(c, entry.Path).Grow(1).MinWidth(0).SingleLine().FontSize(11).Tooltip(entry.Path)
			})
		}).Height(126).MinWidth(0).Label(text(c, "预览文件"))
		if p.EntriesTruncated || p.ReportIncomplete {
			ui.Text(c, text(c, "此列表不完整；执行前请缩小范围或查看日志。")).FontSize(11).TextColor(t.Warning).MaxLines(2)
		}
		ui.Text(c, text(c, "预览反映本次比较结果，执行时文件仍可能发生变化。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
	})
}
