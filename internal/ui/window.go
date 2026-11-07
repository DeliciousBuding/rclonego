package desktop

import (
	"context"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Open(app *core.App, address string) error {
	model := New(app, address)
	mygo.Theme.SetSource(mygo.ThemeSource(model.appearance))
	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{
			Title: "RcloneGo", Width: 1120, Height: 740, MinWidth: 900, MinHeight: 560,
			TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 56,
			Vibrancy: mygo.VibrancyGlass, Content: ui.View(model.view),
			AutoHideMenuBar: true, BackgroundColor: "light-dark(#f5f8f9, #1b282d)",
			UseContentSize: true, DisableFullScreen: true,
		})
		model.Attach(window)
		ctx, cancel := context.WithCancel(context.Background())
		window.OnClosed(cancel)
		model.StartPolling(ctx)
		window.Show()
	})
	return mygo.App.Run()
}
