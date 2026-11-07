package desktop

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
)

func itoa(value int) string { return strconv.Itoa(value) }

//go:embed assets/google-drive.svg
var googleDriveSVG []byte

//go:embed assets/google-g.svg
var googleGSVG []byte

var googleDriveLogo = ui.MustParseSVG(googleDriveSVG)
var googleGLogo = ui.MustParseSVG(googleGSVG)

func brandIcon(c *ui.Context, logo *ui.SVG, size float32) ui.Element {
	return ui.Image(c, logo).Size(size, size).Shrink(0).Role(ui.RoleImage)
}

func remoteLogo(c *ui.Context, kind string, size float32) {
	if kind == "drive" {
		brandIcon(c, googleDriveLogo, size).Label("Google Drive")
		return
	}
	ui.Icon(c, iconRemotes).FontSize(size).TextColor(c.Theme().Accent)
}

// The same 24-unit grid, stroke weight and rounded ends are used everywhere.
// These are application-owned vectors; Apple fonts and symbols aren't bundled.
func lineIcon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	iconOverview   = lineIcon(`<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>`)
	iconRemotes    = lineIcon(`<path d="M6.5 18a4.5 4.5 0 0 1-.8-8.9 6.4 6.4 0 0 1 12.4-1A4.9 4.9 0 0 1 18 18Z"/>`)
	iconMounts     = lineIcon(`<path d="m4 5-2 10v4a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-4L20 5a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2Z"/><path d="M2 15h20m-6 3h.01m3 0h.01"/>`)
	iconQueue      = lineIcon(`<path d="M4 6h16M4 12h12M4 18h8m4-1 3 3 3-3m-3 3v-8"/>`)
	iconRefresh    = lineIcon(`<path d="M20 8a8 8 0 1 0 .6 6M20 3v5h-5"/>`)
	iconPlus       = lineIcon(`<path d="M12 5v14M5 12h14"/>`)
	iconClose      = lineIcon(`<path d="m6 6 12 12M18 6 6 18"/>`)
	iconPause      = lineIcon(`<path d="M8 5v14M16 5v14"/>`)
	iconPlay       = lineIcon(`<path d="m8 5 11 7-11 7Z"/>`)
	iconFile       = lineIcon(`<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9Z"/><path d="M14 3v6h6m-12 5h8m-8 3h5"/>`)
	iconCheck      = lineIcon(`<path d="m5 12 4 4L19 6"/>`)
	iconArrow      = lineIcon(`<path d="M4 12h16m-5-5 5 5-5 5"/>`)
	iconAppearance = lineIcon(`<circle cx="12" cy="12" r="8"/><path d="M12 4a8 8 0 0 1 0 16Z" fill="currentColor" stroke="none"/>`)
	appMark        = lineIcon(`<path d="M9 5H7a4 4 0 0 0-4 4v5a4 4 0 0 0 4 4h2m6-12h2a4 4 0 0 1 4 4v5a4 4 0 0 1-4 4h-2M9 12h6"/>`)
	iconWorkflow   = lineIcon(`<rect x="3" y="3" width="7" height="6" rx="2"/><rect x="14" y="15" width="7" height="6" rx="2"/><path d="M6.5 9v6a3 3 0 0 0 3 3H14"/>`)
	iconFolder     = lineIcon(`<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`)
	iconUp         = lineIcon(`<path d="m6 14 6-6 6 6"/>`)
	iconDown       = lineIcon(`<path d="m6 10 6 6 6-6"/>`)
	iconChevron    = lineIcon(`<path d="m9 6 6 6-6 6"/>`)
	iconResources  = lineIcon(`<path d="M3 12h4l3-8 4 16 3-8h4"/>`)
)

func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(30, 30).Radius(8).Role(ui.RoleButton).Label(label).Tooltip(label)
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
	ui.Row(c).Gap(6).Children(func() {
		ui.Box(c).Size(5, 5).Radius(3).Background(color)
		ui.Text(c, label).FontSize(12).TextColor(color).SingleLine()
	})
}

func sectionHeading(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(15).FontWeight(600)
}

func emptyState(c *ui.Context, icon *ui.SVG, title, detail string) {
	ui.Column(c).Padding(24, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).FontSize(28).TextColor(c.Theme().TextMuted.Alpha(0.65))
		ui.Text(c, title).FontSize(13).FontWeight(500)
		ui.Text(c, detail).FontSize(12).TextColor(c.Theme().TextMuted)
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
	case "oauth-create", "oauth-reconnect":
		return "授权"
	default:
		return operation
	}
}
