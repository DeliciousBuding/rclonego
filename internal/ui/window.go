package desktop

import (
	"context"
	_ "embed"
	"runtime"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed assets/tray.png
var trayIcon []byte

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
		quitting := false
		show := func() { window.Show(); window.Focus() }
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开 RcloneGo", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
			mygo.Separator(),
			{Label: "暂停队列", Click: func(*mygo.MenuItem, *mygo.Window) {
				model.run("暂停队列", func() error { return app.SetPaused(true) })
			}},
			{Label: "继续队列", Click: func(*mygo.MenuItem, *mygo.Window) {
				model.run("继续队列", func() error { return app.SetPaused(false) })
			}},
			mygo.Separator(),
			{Label: "关于 RcloneGo", Click: func(*mygo.MenuItem, *mygo.Window) { model.showAbout = true; show(); window.Invalidate() }},
			{Label: "退出 RcloneGo", Click: func(*mygo.MenuItem, *mygo.Window) { quitting = true; mygo.App.Quit() }},
		})
		tray, err := mygo.NewTray(mygo.TrayOptions{ID: "rclonego", Icon: trayIcon,
			IconIsTemplate: runtime.GOOS == "darwin", ToolTip: "RcloneGo · 可靠传输，过程可见", Menu: menu})
		model.trayAvailable = err == nil
		model.openAtLogin = mygo.App.OpenAtLogin()
		if tray != nil {
			tray.OnClick(show)
			mygo.App.OnQuit(tray.Destroy)
		}
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
		mygo.App.OnActivate(func(bool) { show() })
		window.OnClose(func(event *mygo.CloseEvent) {
			if model.trayAvailable && !quitting {
				event.PreventDefault()
				window.Hide()
			}
		})
		ctx, cancel := context.WithCancel(context.Background())
		window.OnClosed(cancel)
		model.StartPolling(ctx)
		if !mygo.App.WasOpenedAtLogin() || !model.trayAvailable {
			window.Show()
		}
	})
	return mygo.App.Run()
}
