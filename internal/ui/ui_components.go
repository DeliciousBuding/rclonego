package desktop

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func itoa(value int) string { return strconv.Itoa(value) }

// MyGo scrollbars overlay the viewport. Reserve a strip for the visible bar.
func contentScroll(c *ui.Context) ui.Element {
	return ui.Scroll(c).MinWidth(0).MinHeight(0).Padding(0, 16, 0, 0)
}

//go:embed assets/google-drive.svg
var googleDriveSVG []byte

//go:embed assets/google-g.svg
var googleGSVG []byte

//go:embed assets/onedrive.svg
var oneDriveSVG []byte

//go:embed assets/dropbox.svg
var dropboxSVG []byte

//go:embed assets/github.svg
var githubSVG []byte

//go:embed assets/rclone.svg
var rcloneSVG []byte

//go:embed assets/app-mark.svg
var appMarkSVG []byte

var googleDriveLogo = ui.MustParseSVG(googleDriveSVG)
var googleGLogo = ui.MustParseSVG(googleGSVG)
var oneDriveLogo = ui.MustParseSVG(oneDriveSVG)
var dropboxLogo = ui.MustParseSVG(dropboxSVG)
var githubLogo = ui.MustParseSVG(githubSVG)
var rcloneLogo = ui.MustParseSVG(rcloneSVG)
var appMark = ui.MustParseSVG(appMarkSVG)

type providerStyle struct {
	label string
	logo  *ui.SVG
}

var providerStyles = map[string]providerStyle{
	"drive": {"Google Drive", googleDriveLogo}, "onedrive": {"OneDrive", oneDriveLogo},
	"dropbox": {"Dropbox", dropboxLogo}, "s3": {"S3 storage", iconBucket},
	"local": {"本机存储", iconMounts}, "alias": {"位置别名", iconFolder}, "crypt": {"加密存储", iconLock},
	"sftp": {"SFTP", iconServer}, "webdav": {"WebDAV", iconGlobe},
}

func providerName(kind string) string {
	if style, ok := providerStyles[kind]; ok {
		return style.label
	}
	return kind
}

func brandIcon(c *ui.Context, logo *ui.SVG, size float32) ui.Element {
	return ui.Image(c, logo).Size(size, size).Shrink(0).Role(ui.RoleImage)
}

func remoteLogo(c *ui.Context, kind string, size float32) {
	if style, ok := providerStyles[kind]; ok && style.logo != nil {
		brandIcon(c, style.logo, size).Label(text(c, style.label))
		return
	}
	ui.Icon(c, iconRemotes).FontSize(size).TextColor(c.Theme().Accent)
}

func (s *nativeUI) providerForPaths(paths ...string) string {
	for _, path := range paths {
		for _, remote := range s.remotes {
			if strings.HasPrefix(path, remote.Name+":") || strings.HasPrefix(path, remote.Name+"{") {
				return remote.Type
			}
		}
	}
	return ""
}

func (s *nativeUI) storageIcon(c *ui.Context, path string, fallback *ui.SVG, size float32) {
	if kind := s.providerForPaths(path); kind != "" {
		remoteLogo(c, kind, size)
		return
	}
	ui.Icon(c, fallback).FontSize(size).TextColor(c.Theme().TextMuted)
}

// The same 24-unit grid, stroke weight and rounded ends are used everywhere.
// These are application-owned vectors; Apple fonts and symbols aren't bundled.
func lineIcon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	iconOverview  = lineIcon(`<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>`)
	iconRemotes   = lineIcon(`<path d="M6.5 18a4.5 4.5 0 0 1-.8-8.9 6.4 6.4 0 0 1 12.4-1A4.9 4.9 0 0 1 18 18Z"/>`)
	iconMounts    = lineIcon(`<path d="m4 5-2 10v4a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-4L20 5a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2Z"/><path d="M2 15h20m-6 3h.01m3 0h.01"/>`)
	iconQueue     = lineIcon(`<path d="M4 6h16M4 12h12M4 18h8m4-1 3 3 3-3m-3 3v-8"/>`)
	iconRefresh   = lineIcon(`<path d="M20 8a8 8 0 1 0 .6 6M20 3v5h-5"/>`)
	iconSearch    = lineIcon(`<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`)
	iconPlus      = lineIcon(`<path d="M12 5v14M5 12h14"/>`)
	iconClose     = lineIcon(`<path d="m6 6 12 12M18 6 6 18"/>`)
	iconPause     = lineIcon(`<path d="M8 5v14M16 5v14"/>`)
	iconPlay      = lineIcon(`<path d="m8 5 11 7-11 7Z"/>`)
	iconFile      = lineIcon(`<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9Z"/><path d="M14 3v6h6m-12 5h8m-8 3h5"/>`)
	iconCheck     = lineIcon(`<path d="m5 12 4 4L19 6"/>`)
	iconArrow     = lineIcon(`<path d="M4 12h16m-5-5 5 5-5 5"/>`)
	iconSettings  = lineIcon(`<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="9" cy="6" r="2" fill="currentColor"/><circle cx="15" cy="12" r="2" fill="currentColor"/><circle cx="9" cy="18" r="2" fill="currentColor"/>`)
	iconMore      = lineIcon(`<circle cx="5" cy="12" r="1" fill="currentColor"/><circle cx="12" cy="12" r="1" fill="currentColor"/><circle cx="19" cy="12" r="1" fill="currentColor"/>`)
	iconRename    = lineIcon(`<path d="m15 4 5 5M4 20l4-1L20 7a2 2 0 0 0-4-4L4 15Z"/>`)
	iconWorkflow  = lineIcon(`<rect x="3" y="3" width="7" height="6" rx="2"/><rect x="14" y="15" width="7" height="6" rx="2"/><path d="M6.5 9v6a3 3 0 0 0 3 3H14"/>`)
	iconFolder    = lineIcon(`<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`)
	iconUp        = lineIcon(`<path d="m6 14 6-6 6 6"/>`)
	iconDown      = lineIcon(`<path d="m6 10 6 6 6-6"/>`)
	iconChevron   = lineIcon(`<path d="m9 6 6 6-6 6"/>`)
	iconResources = lineIcon(`<path d="M3 12h4l3-8 4 16 3-8h4"/>`)
	iconDownload  = lineIcon(`<path d="M12 3v12m-5-5 5 5 5-5M4 17v3h16v-3"/>`)
	iconEngine    = lineIcon(`<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3m6 0h4"/>`)
	iconInfo      = lineIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10h.01"/>`)
	iconTrash     = lineIcon(`<path d="M3 6h18m-13 0V3h8v3m-11 0 1 15h12l1-15m-9 4v7m4-7v7"/>`)
	iconWarning   = lineIcon(`<path d="m12 3 10 18H2Z"/><path d="M12 9v5m0 3h.01"/>`)
	iconSchedule  = lineIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`)
	iconBucket    = lineIcon(`<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="m4 5 2 14c1 3 11 3 12 0l2-14M5 12c2 3 12 3 14 0"/>`)
	iconLock      = lineIcon(`<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 4v3"/>`)
	iconServer    = lineIcon(`<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01m5-11h5m-5 11h5"/>`)
	iconGlobe     = lineIcon(`<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>`)
)

func actionButton(c *ui.Context, icon *ui.SVG, label string, primary bool) ui.Element {
	label = text(c, label)
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c, "")
	} else {
		b = ui.Button(c, "")
	}
	b.Height(30).Padding(0, 10).Gap(7).Shrink(0).Label(label).Children(func() {
		ui.Icon(c, icon).FontSize(15)
		ui.Text(c, label).FontSize(12).SingleLine()
	})
	return b
}

func (s *nativeUI) pathInput(c *ui.Context, value *string, label, placeholder string, directory, disabled bool, filters []mygo.FileFilter) {
	ui.Row(c).Gap(6).MinWidth(0).Children(func() {
		ui.TextInput(c.Key("value"), value).Grow(1).MinWidth(0).Label(text(c, label)).Placeholder(text(c, placeholder)).Disabled(disabled)
		iconButton(c.Key("browse"), iconFolder, fmt.Sprintf(text(c, "浏览%s"), text(c, label))).Disabled(s.busy || disabled).OnClick(func() {
			defaultPath := *value
			dialogTitle := fmt.Sprintf(text(c, "选择%s"), text(c, label))
			go func() {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: s.win, Title: dialogTitle, DefaultPath: defaultPath, Directory: directory, Filters: filters})
				s.win.Update(func() {
					if err != nil {
						s.setError(err)
					} else if len(paths) > 0 {
						*value = paths[0]
					}
				})
			}()
		})
	})
}
func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	label = text(c, label)
	t := c.Theme()
	b := ui.ButtonBase(c).Size(30, 30).Radius(8).Role(ui.RoleButton).Label(label).Tooltip(label)
	b.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icon).FontSize(17).TextColor(t.TextMuted)
	})
	return b
}

func statusDot(c *ui.Context, label string, color ui.Color) {
	label = text(c, label)
	ui.Row(c).Gap(6).Children(func() {
		ui.Box(c).Size(5, 5).Radius(3).Background(color)
		ui.Text(c, label).FontSize(12).TextColor(color).SingleLine()
	})
}

func sectionHeading(c *ui.Context, title string) {
	ui.Text(c, text(c, title)).FontSize(15).FontWeight(600)
}

func pathSummary(c *ui.Context, source, destination string) {
	route := source + " → " + destination
	ui.Text(c, route).FontSize(11).TextColor(c.Theme().TextMuted).
		SingleLine().Tooltip(route)
}

func pageHeading(c *ui.Context, title, description string, actions func()) {
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, text(c, title)).FontSize(22).FontWeight(600).SingleLine()
			ui.Text(c, text(c, description)).FontSize(12).TextColor(c.Theme().TextMuted).MaxLines(2)
		})
		if actions != nil {
			ui.Row(c).Gap(8).Shrink(0).Children(actions)
		}
	})
}

func emptyState(c *ui.Context, icon *ui.SVG, title, detail string) {
	ui.Column(c).Padding(24, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).FontSize(28).TextColor(c.Theme().TextMuted.Alpha(0.65))
		ui.Text(c, text(c, title)).FontSize(13).FontWeight(500)
		ui.Text(c, text(c, detail)).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func formatBytes(n int64) string {
	if n < 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit || suffix == "TiB" {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return "—"
}

func progressValue(done, total int64) float64 {
	if total <= 0 {
		return -1
	}
	return math.Max(0, math.Min(1, float64(done)/float64(total)))
}

func operationLabel(operation string) string {
	switch operation {
	case "copy":
		return "复制"
	case "copyto":
		return "复制文件"
	case "sync":
		return "同步"
	case "check":
		return "校验"
	case "mkdir":
		return "新建文件夹"
	case "moveto":
		return "重命名"
	case "deletefile", "rmdir":
		return "删除文件"
	case "oauth-create", "oauth-reconnect":
		return "授权"
	default:
		return operation
	}
}
