package desktop

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/DeliciousBuding/rclonego/locales"
	"runtime"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed assets/tray.png
var trayIcon []byte

func Open(app *core.App, address string) error {
	model := New(app, address)
	mygo.App.SetPath(mygo.PathUserData, app.DataDirectory())
	configureUpdates(app)
	mygo.Theme.SetSource(mygo.ThemeSource(model.appearance))
	mygo.App.WhenReady(func() {
		// Keep system decorations on Windows/Linux. macOS traffic lights
		// share an inset toolbar with a reserved native control area.
		titleBarStyle := mygo.TitleBarDefault
		material := mygo.Vibrancy("")
		if runtime.GOOS == "darwin" {
			titleBarStyle, material = mygo.TitleBarHiddenInset, mygo.VibrancyGlass
		}
		if runtime.GOOS == "windows" {
			material = mygo.VibrancyMica
		}
		window := mygo.NewWindow(mygo.WindowOptions{
			Title: "RcloneGo", Width: 1120, Height: 740, MinWidth: 900, MinHeight: 560,
			TitleBarStyle: titleBarStyle,
			Vibrancy:      material, Content: ui.View(model.view),
			AutoHideMenuBar: true, BackgroundColor: "light-dark(#eef2f5, #222a32)",
			UseContentSize: true, DisableFullScreen: true,
			Hidden: true,
		})
		model.Attach(window)
		removeLanguagePreferenceListener := mygo.App.OnLocaleChanged(func(string) {
			model.languageOverride = savedLanguagePreference(app.DataDirectory())
			if !model.isError {
				model.notice = ""
			}
			window.Invalidate()
		})
		window.OnClosed(removeLanguagePreferenceListener)
		quitting, explicitQuit := false, false
		show := func() { window.Show(); window.Focus() }
		removeInstanceListener := mygo.App.OnSecondInstance(func([]string, string) { show() })
		window.OnClosed(removeInstanceListener)
		quit := func() { explicitQuit = true; mygo.App.Quit() }
		makeApplicationMenu := func() *mygo.Menu {
			applicationItems := []*mygo.MenuItem{
				{Label: locales.AboutRcloneGo(), Click: func(*mygo.MenuItem, *mygo.Window) { model.showAbout = true; show() }},
				{Label: locales.Text("设置"), Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { model.page = "settings"; show() }},
				updatesMenuItem(),
				mygo.Separator(),
			}
			if runtime.GOOS == "darwin" {
				applicationItems = append(applicationItems, &mygo.MenuItem{Role: mygo.RoleServices},
					mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleHide},
					&mygo.MenuItem{Role: mygo.RoleHideOthers}, &mygo.MenuItem{Role: mygo.RoleUnhide},
					mygo.Separator())
			}
			applicationItems = append(applicationItems, &mygo.MenuItem{
				Label: locales.QuitRcloneGo(), Accelerator: "CmdOrCtrl+Q",
				Click: func(*mygo.MenuItem, *mygo.Window) { quit() },
			})
			application := &mygo.MenuItem{Label: "RcloneGo", Submenu: applicationItems}
			items := []*mygo.MenuItem{application}
			if runtime.GOOS == "darwin" {
				items = append(items, &mygo.MenuItem{Role: mygo.RoleEditMenu}, &mygo.MenuItem{Role: mygo.RoleWindowMenu})
			}
			return mygo.NewMenu(items)
		}
		mygo.App.SetMenu(makeApplicationMenu())
		removeMenuLanguageListener := mygo.App.OnLocaleChanged(func(string) {
			mygo.App.SetMenu(makeApplicationMenu())
		})
		window.OnClosed(removeMenuLanguageListener)
		makeMenu := func() *mygo.Menu {
			return mygo.NewMenu([]*mygo.MenuItem{
				{Label: locales.OpenRcloneGo(), Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
				mygo.Separator(),
				{Label: locales.PauseQueue(), Click: func(*mygo.MenuItem, *mygo.Window) {
					model.run(locales.PauseQueue(), func() error { return app.SetPaused(true) })
				}},
				{Label: locales.ResumeQueue(), Click: func(*mygo.MenuItem, *mygo.Window) {
					model.run(locales.ResumeQueue(), func() error { return app.SetPaused(false) })
				}},
				mygo.Separator(),
				{Label: locales.AboutRcloneGo(), Click: func(*mygo.MenuItem, *mygo.Window) { model.showAbout = true; show(); window.Invalidate() }},
				updatesMenuItem(),
				{Label: locales.QuitRcloneGo(), Click: func(*mygo.MenuItem, *mygo.Window) { quit() }},
			})
		}
		tray, err := mygo.NewTray(mygo.TrayOptions{ID: "rclonego", Icon: trayIcon,
			IconIsTemplate: runtime.GOOS == "darwin", ToolTip: locales.TrayTooltip(), Menu: makeMenu()})
		model.trayAvailable = err == nil
		model.openAtLogin = model.canConfigureLogin() && mygo.App.OpenAtLogin()
		if tray != nil {
			tray.OnClick(show)
			mygo.App.OnQuit(tray.Destroy)
			removeLocaleListener := mygo.App.OnLocaleChanged(func(string) {
				tray.SetMenu(makeMenu())
				tray.SetToolTip(locales.TrayTooltip())
			})
			window.OnClosed(removeLocaleListener)
		}
		mygo.App.OnBeforeQuit(func(event *mygo.QuitEvent) {
			if !explicitQuit && updateRestartBlocked(app) {
				event.PreventDefault()
				model.setError(fmt.Errorf("%s", locales.Text("任务或映射正在使用，请在空闲后重启完成更新")))
				show()
				return
			}
			quitting = true
		})
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
