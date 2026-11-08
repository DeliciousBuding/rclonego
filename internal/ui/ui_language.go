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

// MyGo owns the current language, system language matching and persistence.
// The picker shows the effective language; the separate reset action restores
// system matching without duplicating MyGo's saved preference.
func (s *nativeUI) languagePreferences(c *ui.Context) {
	language := c.Locale()
	selected := i18n.Autonym(language)
	ui.Column(c.Key("language-preferences")).Gap(8).Children(func() {
		sectionHeading(c, locales.In(language).Language())
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.Select(c.Key("language"), &selected, []string{"English", "简体中文"}).
				Width(180).Label(locales.In(language).Language()).Changed() {
				tag := "en"
				if selected == "简体中文" {
					tag = "zh-Hans"
				}
				mygo.App.SetLocale(tag)
			}
			ui.Button(c.Key("system-language"), locales.In(language).UseSystemLanguage()).
				OnClick(func() { mygo.App.SetLocale("") })
		})
		ui.Text(c, locales.In(language).LanguageDescription()).
			FontSize(12).TextColor(c.Theme().TextMuted).MaxLines(2)
	})
}
