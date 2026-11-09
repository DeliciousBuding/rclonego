package desktop

import (
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var setWindowFrameAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

func configureNativeFrame(window *mygo.Window) {
	update := func() { updateNativeFrame(window) }
	update()
	window.OnClosed(mygo.Theme.OnUpdated(update))
}

func updateNativeFrame(window *mygo.Window) {
	if window == nil {
		return
	}
	mygo.RunOnMain(func() {
		dark := mygo.Theme.IsDark()
		highContrast := mygo.Theme.Preferences().HighContrast
		theme := modernTheme(dark, ui.Preferences{})
		for _, attribute := range []struct {
			id    uintptr
			color ui.Color
		}{
			{35, windowChrome(dark)}, // DWMWA_CAPTION_COLOR
			{36, theme.Text},         // DWMWA_TEXT_COLOR
		} {
			color := uint32(attribute.color.R) | uint32(attribute.color.G)<<8 | uint32(attribute.color.B)<<16
			if highContrast {
				color = 0xffffffff // DWMWA_COLOR_DEFAULT
			}
			// Windows 11 colors its native caption without replacing the
			// buttons. Older Windows keeps its system caption treatment.
			setWindowFrameAttribute.Call(window.NativeHandle(), attribute.id, uintptr(unsafe.Pointer(&color)), 4)
		}
	})
}
