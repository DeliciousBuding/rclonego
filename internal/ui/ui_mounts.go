package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
	"strings"
)

func (s *nativeUI) mountEditor(c *ui.Context) {
	ui.Form(c, func() {
		ui.Field(c, "名称", func() { ui.TextInput(c.Key("mount-name"), &s.mountName).AutoFocus() })
		ui.Field(c, "远端", func() { ui.TextInput(c.Key("mount-remote"), &s.mountRemote).Placeholder("drive:") })
		ui.Field(c, "盘符", func() { ui.TextInput(c.Key("mount-drive"), &s.mountDrive).Placeholder("K:") })
		ui.Field(c, "缓存目录", func() { ui.TextInput(c.Key("mount-cache"), &s.mountCache).Placeholder("选择本机 SSD 目录") })
	})
	ui.Collapsible(c.Key("mount-advanced"), "更多选项", &s.showMountAdvanced, func() {
		ui.Form(c, func() {
			ui.Field(c, "卷标", func() { ui.TextInput(c.Key("mount-volume"), &s.mountVolume) })
			ui.Field(c, "缓存上限", func() { ui.TextInput(c.Key("mount-max"), &s.mountMax).Placeholder("15G") })
			ui.Field(c, "缓存保留时间", func() { ui.TextInput(c.Key("mount-age"), &s.mountAge).Placeholder("24h") })
		})
	})
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		ui.Button(c.Key("cancel-mount"), "取消").Height(30).OnClick(func() { s.editingMount = false })
		ui.PrimaryButton(c.Key("save-mount"), "保存").Height(30).Disabled(s.busy).OnClick(s.saveMount)
	})
}

func (s *nativeUI) saveMount() {
	cfg := core.MountConfig{ID: s.mountID, Name: strings.TrimSpace(s.mountName), Remote: core.EnsureColonRemote(strings.TrimSpace(s.mountRemote)), Drive: strings.ToUpper(strings.TrimSpace(s.mountDrive)), VolumeName: strings.TrimSpace(s.mountVolume), CacheDir: strings.TrimSpace(s.mountCache), CacheMax: strings.TrimSpace(s.mountMax), CacheMaxAge: strings.TrimSpace(s.mountAge)}
	if cfg.ID == "" {
		id, err := core.RandomID()
		if err != nil {
			s.setError(err)
			return
		}
		cfg.ID = "mount-" + id[:6]
	}
	s.runAfter("保存映射", func() error {
		if err := core.ValidateMountConfig(cfg); err != nil {
			return err
		}
		return s.app.UpdateState(func(state *core.State) error {
			for i := range state.Mounts {
				if state.Mounts[i].ID == cfg.ID {
					state.Mounts[i] = cfg
					return nil
				}
				if strings.EqualFold(state.Mounts[i].Drive, cfg.Drive) {
					return fmt.Errorf("drive %s is already mapped", cfg.Drive)
				}
			}
			state.Mounts = append(state.Mounts, cfg)
			return nil
		})
	}, func() { s.editingMount = false })
}
