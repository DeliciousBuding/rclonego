package locales

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/egoist/mygo/i18n"
)

var sourceMessages = sync.OnceValue(func() map[string]string {
	data, err := files.ReadFile("zh-Hans.json")
	if err != nil {
		panic(err)
	}
	var messages map[string]json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		panic(err)
	}
	keys := make(map[string]string, len(messages))
	for key, raw := range messages {
		if strings.HasPrefix(key, "@") {
			continue
		}
		var source string
		if json.Unmarshal(raw, &source) == nil {
			keys[source] = key
		}
	}
	return keys
})

// TextIn translates a complete static source message through MyGo's catalog.
// Unknown text is returned unchanged, so names, paths and engine logs are never
// rewritten. Translate format strings before supplying their arguments.
// New UI code can use the generated typed accessors directly.
func TextIn(language, source string) string {
	key, ok := sourceMessages()[source]
	if !ok {
		return source
	}
	return Catalog.Format(language, key, nil)
}

// Text says a static source message in the application's current language.
func Text(source string) string {
	return TextIn(i18n.Locale(), source)
}
