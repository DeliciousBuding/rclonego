package desktop

import (
	"os"
	"strings"

	"github.com/egoist/mygo/i18n"
)

// MyGo 0.8.3 exposes the effective language but not the saved override. Read
// its existing preference only to display the picker mode. Matching, writes,
// system events and the native menus continue to use MyGo's locale APIs.
func savedLanguagePreference(dataDirectory string) string {
	if tag := strings.TrimSpace(os.Getenv("MYGO_LOCALE")); tag != "" {
		return i18n.Normalize(tag)
	}
	return i18n.Normalize(platformLanguagePreference(dataDirectory))
}
