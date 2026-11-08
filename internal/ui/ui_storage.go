package desktop

import (
	"context"
	"fmt"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

type storageUIState struct {
	info        core.StorageInfo
	quota       core.RemoteQuota
	loading     bool
	err         string
	detailsOpen bool
}

func (s *nativeUI) loadStorage(name string) {
	if s.storageUI == nil {
		s.storageUI = map[string]*storageUIState{}
	}
	current := s.storageUI[name]
	if current != nil && current.loading {
		return
	}
	next := &storageUIState{loading: true}
	if current != nil {
		next.detailsOpen = current.detailsOpen
	}
	s.storageUI[name] = next
	go func() {
		info, err := s.app.StorageInfo(context.Background(), name)
		var quota core.RemoteQuota
		if err == nil {
			if info.Capabilities.Quota {
				quota, err = s.app.RemoteQuota(context.Background(), name)
			} else {
				quota = core.RemoteQuota{Name: name, Status: "unsupported", UpdatedAt: info.UpdatedAt}
			}
		}
		s.win.Update(func() {
			if s.storageUI[name] != next {
				return
			}
			next.info, next.quota, next.loading = info, quota, false
			if err != nil {
				next.err = err.Error()
			}
		})
	}()
}

func quotaSummary(c *ui.Context, quota core.RemoteQuota) string {
	used := quotaOccupied(quota)
	switch {
	case quota.Status == "unsupported":
		return text(c, "服务未提供账户容量")
	case used != nil && quota.Total != nil:
		return fmt.Sprintf(text(c, "已用 %s / %s"), formatBytes(*used), formatBytes(*quota.Total))
	case quota.Free != nil:
		return fmt.Sprintf(text(c, "剩余 %s"), formatBytes(*quota.Free))
	case used != nil:
		return fmt.Sprintf(text(c, "已用 %s"), formatBytes(*used))
	case quota.Total != nil:
		return fmt.Sprintf(text(c, "总容量 %s"), formatBytes(*quota.Total))
	default:
		return text(c, "容量尚未读取")
	}
}

// Google Drive reports file usage separately from other Google services.
// When total and free are available, their difference is account occupancy.
func quotaOccupied(quota core.RemoteQuota) *int64 {
	if quota.Total != nil && quota.Free != nil && *quota.Total >= *quota.Free {
		value := *quota.Total - *quota.Free
		return &value
	}
	return quota.Used
}

func (s *nativeUI) storageDetails(c *ui.Context) {
	t := c.Theme()
	var remote core.RemoteInfo
	for _, candidate := range s.remotes {
		if candidate.Name == s.storageDetail {
			remote = candidate
		}
	}
	ui.Row(c).Gap(12).Children(func() {
		remoteLogo(c, remote.Type, 30)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, remote.Name).FontSize(17).FontWeight(600).SingleLine().Tooltip(remote.Name)
			ui.Text(c, text(c, providerName(remote.Type))).FontSize(12).TextColor(t.TextMuted)
		})
		iconButton(c.Key("refresh-storage"), iconRefresh, "刷新账户信息").
			Disabled(s.storageUI[remote.Name] != nil && s.storageUI[remote.Name].loading).
			OnClick(func() { s.loadStorage(remote.Name) })
	})
	state := s.storageUI[remote.Name]
	if state == nil || state.loading {
		ui.Text(c, text(c, "正在读取账户信息…")).FontSize(12).TextColor(t.TextMuted)
		return
	}
	if state.err != "" {
		ui.Text(c, text(c, "账户信息读取失败")).FontSize(13).FontWeight(500).TextColor(t.Warning)
		ui.Text(c, state.err).FontSize(12).MaxLines(4).Selectable()
	}
	sectionHeading(c, "账户容量")
	ui.Text(c, quotaSummary(c, state.quota)).FontSize(13).FontWeight(500)
	occupied := quotaOccupied(state.quota)
	if state.quota.Total != nil && *state.quota.Total > 0 && occupied != nil {
		ui.Progress(c, progressValue(*occupied, *state.quota.Total)).Height(4).Radius(2).Label(text(c, "账户空间占用"))
	}
	ui.Row(c).Gap(22).Wrap().Children(func() {
		for _, item := range []struct {
			label string
			value *int64
		}{{"总容量", state.quota.Total}, {"已用空间", occupied}, {"剩余空间", state.quota.Free}} {
			value := text(c, "未提供")
			if item.value != nil {
				value = formatBytes(*item.value)
			}
			s.metric(c, item.label, value)
		}
	})
	if state.quota.Used != nil && occupied != nil && *occupied != *state.quota.Used {
		ui.Text(c, text(c, "存储文件")+" · "+formatBytes(*state.quota.Used)).FontSize(11).TextColor(t.TextMuted)
	}
	for _, item := range []struct {
		label string
		value *int64
	}{{"回收站", state.quota.Trashed}, {"其他服务", state.quota.Other}} {
		if item.value != nil {
			ui.Text(c, text(c, item.label)+" · "+formatBytes(*item.value)).FontSize(11).TextColor(t.TextMuted)
		}
	}
	ui.Text(c, text(c, "服务返回的账户配额；可能包含邮件、照片或回收站。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
	if !state.quota.UpdatedAt.IsZero() {
		ui.Text(c, text(c, "更新于 ")+dateTime(c, state.quota.UpdatedAt)).FontSize(11).TextColor(t.TextMuted)
	}
	if state.info.UpdatedAt.IsZero() {
		return
	}
	ui.Divider(c)
	hashes := strings.Join(state.info.Hashes, ", ")
	if hashes == "" {
		hashes = text(c, "未提供")
	}
	ui.Text(c, text(c, "校验算法")+" · "+hashes).FontSize(12).MaxLines(3).Tooltip(hashes).Selectable()
	ui.Text(c, text(c, "校验仍取决于两端共有的算法；复制完成不代表内容校验完成。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
	ui.Collapsible(c.Key("backend-capabilities"), text(c, "后端能力"), &state.detailsOpen, func() {
		f := state.info.Capabilities
		for _, item := range []struct {
			label string
			value bool
		}{{"容量查询", f.Quota}, {"服务端复制", f.ServerSideCopy}, {"服务端移动", f.ServerSideMove},
			{"分享链接", f.PublicLink}, {"空目录", f.EmptyDirectories}, {"递归列举", f.RecursiveListing},
			{"读取元数据", f.ReadMetadata}, {"写入元数据", f.WriteMetadata}, {"大小写不敏感", f.CaseInsensitive}} {
			ui.Row(c).Gap(10).Children(func() {
				ui.Text(c, text(c, item.label)).Grow(1).FontSize(12)
				value, icon, color := "未支持", iconClose, t.TextMuted
				if item.value {
					value, icon, color = "支持", iconCheck, t.Success
				}
				ui.Icon(c, icon).FontSize(13).TextColor(color)
				ui.Text(c, text(c, value)).FontSize(11).TextColor(color)
			})
		}
		ui.Text(c, text(c, "能力来自当前 rclone 后端；具体操作仍受账户权限和服务限制。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
	})
}
