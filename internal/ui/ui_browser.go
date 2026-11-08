package desktop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

type browserUIState struct {
	open       bool
	target     *string
	path       string
	remote     string
	result     core.BrowseResult
	offset     int
	loading    bool
	err        string
	cancel     context.CancelFunc
	generation int
	list       ui.ListState
}

func (s *nativeUI) storagePathInput(c *ui.Context, value *string, label string, disabled bool) {
	ui.Row(c).MinWidth(0).Gap(6).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			s.pathInput(c.Key("local"), value, label, "本机目录或 remote:目录", true, disabled, nil)
		})
		iconButton(c.Key("cloud"), iconRemotes, fmt.Sprintf(text(c, "选择云端%s"), text(c, label))).Disabled(s.busy || disabled || len(s.remotes) == 0).
			OnClick(func() { s.beginBrowse(value) })
	})
}

func (s *nativeUI) beginBrowse(target *string) {
	b := &s.browserUI
	if b.cancel != nil {
		b.cancel()
	}
	b.open, b.target, b.offset = true, target, 0
	b.path = *target
	b.remote = ""
	for _, remote := range s.remotes {
		name := strings.TrimSuffix(remote.Name, ":") + ":"
		if b.remote == "" {
			b.remote = name
		}
		if strings.HasPrefix(*target, name) {
			b.remote, b.path = name, *target
			break
		}
	}
	if !strings.HasPrefix(b.path, b.remote) || b.path == "" {
		b.path = b.remote
	}
	s.loadBrowser(b.path, 0)
}

func (s *nativeUI) closeBrowser() {
	b := &s.browserUI
	b.open = false
	b.generation++
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

func (s *nativeUI) loadBrowser(path string, offset int) {
	b := &s.browserUI
	if b.cancel != nil {
		b.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.cancel, b.path, b.offset, b.loading, b.err = cancel, path, offset, true, ""
	b.generation++
	generation := b.generation
	go func() {
		defer cancel()
		result, err := s.app.Browse(ctx, core.BrowseRequest{Path: path, Offset: offset, Limit: 100, DirectoriesOnly: true})
		s.win.Update(func() {
			if !b.open || generation != b.generation {
				return
			}
			b.loading = false
			if err != nil {
				b.err = err.Error()
				return
			}
			b.result = result
		})
	}()
}

func (s *nativeUI) browserOverlay(c *ui.Context) {
	b := &s.browserUI
	ui.DialogBase(c.Key("directory-browser"), &b.open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(560).MaxWidthPercent(90).MaxHeightPercent(88).Padding(20).Gap(12).Radius(16).Background(t.Background)
		ui.Row(c).Children(func() {
			sectionHeading(c, "选择云端目录")
			ui.Spacer(c)
			iconButton(c.Key("close"), iconClose, "关闭目录选择").OnClick(s.closeBrowser)
		})
		var remotes []string
		for _, remote := range s.remotes {
			remotes = append(remotes, strings.TrimSuffix(remote.Name, ":")+":")
		}
		if ui.Select(c.Key("remote"), &b.remote, remotes).Label(text(c, "云端账户")).Disabled(b.loading).Changed() {
			s.loadBrowser(b.remote, 0)
		}
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c.Key("path"), &b.path).Grow(1).MinWidth(0).Label(text(c, "云端路径")).Disabled(b.loading)
			iconButton(c.Key("open-path"), iconArrow, "打开目录").Disabled(b.loading).OnClick(func() { s.loadBrowser(b.path, 0) })
		})
		ui.Row(c).Gap(8).Children(func() {
			iconButton(c.Key("parent"), iconUp, "上一级目录").Disabled(b.loading || b.result.Parent == "").OnClick(func() { s.loadBrowser(b.result.Parent, 0) })
			ui.Text(c, b.result.Path).Grow(1).MinWidth(0).SingleLine().FontSize(11).TextColor(t.TextMuted).Tooltip(b.result.Path)
			iconButton(c.Key("refresh"), iconRefresh, "刷新目录").Disabled(b.loading).OnClick(func() { s.loadBrowser(b.path, b.offset) })
		})
		if b.loading {
			ui.Text(c, text(c, "正在读取目录…")).FontSize(12).TextColor(t.TextMuted)
		} else if b.err != "" {
			ui.Text(c, b.err).FontSize(12).TextColor(t.Danger).MaxLines(3)
		} else if len(b.result.Entries) == 0 {
			ui.Text(c, text(c, "此目录没有子目录")).FontSize(12).TextColor(t.TextMuted)
		}
		b.list.Key = func(i int) any { return b.result.Entries[i].Path }
		b.list.Label = func(i int) string { return b.result.Entries[i].Name }
		ui.List(c.Key("directories"), &b.list, len(b.result.Entries), func(i int) {
			entry := b.result.Entries[i]
			ui.Row(c).Gap(9).MinWidth(0).Children(func() {
				ui.Icon(c, iconFolder).FontSize(18).TextColor(t.Accent)
				ui.Text(c, entry.Name).Grow(1).MinWidth(0).SingleLine().Tooltip(entry.Name)
				iconButton(c.Key("enter"), iconChevron, "打开子目录").Disabled(b.loading).OnClick(func() { s.loadBrowser(entry.Path, 0) })
			})
		}).Height(252).MinWidth(0).Label(text(c, "云端子目录"))
		if b.result.Truncated {
			ui.Text(c, text(c, "目录过大，当前显示前一万项中的结果；可输入更具体的路径。")).FontSize(11).TextColor(t.Warning).MaxLines(2)
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c.Key("previous"), text(c, "上一页")).Disabled(b.loading || b.offset == 0).OnClick(func() { s.loadBrowser(b.path, max(0, b.offset-100)) })
			ui.Button(c.Key("next"), text(c, "下一页")).Disabled(b.loading || !b.result.HasMore).OnClick(func() { s.loadBrowser(b.path, b.result.NextOffset) })
			ui.Spacer(c)
			ui.PrimaryButton(c.Key("select"), text(c, "选择此目录")).Disabled(b.loading || b.err != "" || b.result.Path == "" || b.path != b.result.Path).OnClick(func() {
				if b.target != nil {
					*b.target = b.result.Path
					s.jobPreview = core.Preview{}
				}
				s.closeBrowser()
			})
		})
	})
	if !b.open && b.cancel != nil {
		s.closeBrowser()
	}
}
