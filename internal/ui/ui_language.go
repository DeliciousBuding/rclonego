package desktop

import (
	"fmt"
	"math"
	"time"

	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// text is for complete static interface messages. Names, paths, provider
// questions and engine logs keep their original content.
func text(c *ui.Context, source string) string {
	return locales.TextIn(c.Locale(), source)
}

func dateTime(c *ui.Context, value time.Time) string {
	language := c.Region().FormatLocale(c.Locale())
	return i18n.FormatDate(language, value.Local(), i18n.DateOptions{
		Date: i18n.MediumStyle,
		Time: i18n.ShortStyle,
	})
}

// translatedSelect retains the application's canonical values while MyGo
// renders translated option labels. Changing language never changes saved
// operations, filters, verification modes or theme selection.
func translatedSelect(c *ui.Context, selected *string, options []string) ui.Element {
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = text(c, option)
	}
	display := text(c, *selected)
	control := ui.Select(c, &display, labels)
	if control.Changed() {
		for i, label := range labels {
			if label == display {
				*selected = options[i]
				break
			}
		}
	}
	return control
}

func durationText(c *ui.Context, seconds float64) string {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return "—"
	}
	format := func(source string, values ...any) string {
		return fmt.Sprintf(text(c, source), values...)
	}
	if seconds < 60 {
		return format("%d 秒", int(seconds))
	}
	minutes := int(seconds / 60)
	if minutes < 60 {
		return format("%d 分钟", minutes)
	}
	hours := minutes / 60
	if hours >= 24 {
		return format("%d 天 %d 小时", hours/24, hours%24)
	}
	return format("%d 小时 %d 分", hours, minutes%60)
}

// MyGo owns language matching and persistence. languageOverride only remembers
// the picker mode for this window; it is loaded from MyGo's existing preference.
func (s *nativeUI) languagePreferences(c *ui.Context) {
	settingsPreferenceRow(c.Key("language-preferences"), locales.In(c.Locale()).Language(),
		locales.In(c.Locale()).LanguageDescription(), func() {
			languagePicker(c, &s.languageOverride, mygo.App.SetLocale)
			ui.Text(c, locales.In(c.Locale()).CurrentLanguage(i18n.Autonym(c.Locale()))).
				FontSize(11).TextColor(c.Theme().TextMuted)
		})
}

func languagePicker(c *ui.Context, preference *string, apply func(string)) {
	system := text(c, "跟随系统")
	selected := system
	if *preference != "" {
		selected = i18n.Autonym(c.Locale())
	}
	if ui.Select(c.Key("language"), &selected, []string{system, "简体中文", "English"}).
		FillWidth().Label(locales.In(c.Locale()).Language()).Changed() {
		tag := ""
		if selected == "English" {
			tag = "en"
		} else if selected == "简体中文" {
			tag = "zh-Hans"
		}
		*preference = tag
		apply(tag)
	}
}

func settingsPreferenceRow(c *ui.Context, label, description string, controls func()) {
	ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(13)
			if description != "" {
				ui.Text(c, description).FontSize(11).TextColor(c.Theme().TextMuted).MaxLines(3)
			}
		})
		ui.Column(c).Width(220).Shrink(0).Gap(4).Children(controls)
	})
}
