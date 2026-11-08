# Interface languages

RcloneGo's language resources use the official MyGo catalogs and typed Go
accessors. The catalogs cover English and Simplified Chinese. MyGo chooses the
best supported language from the operating system preferences.

The language selector in **Settings** changes the interface immediately. **Use system
language** restores automatic matching. MyGo persists this choice using the
platform's application preferences; RcloneGo does not create a second language
setting. Native controls and the official update window follow the same
application language.

Task names, paths, provider questions and rclone logs retain their original
content. Display language does not change API field names, command names,
operation identifiers or saved workflow data.

## Contributing translations

Messages live in `locales/en.json` and `locales/zh-Hans.json`. English is the
source catalog. Add the same key to both languages, use a short semantic name,
and preserve arguments. Counts and dates use ICU MessageFormat:

```json
{
  "filesCount": "{count, plural, one {# file} other {# files}}"
}
```

Use generated accessors for new interface messages:

```go
ui.Text(c, locales.In(c.Locale()).FilesCount(len(files)))
```

The existing static-message adapter translates complete source messages through
the same catalog. Unknown strings pass through unchanged. Translate a format
string before formatting it, and pass dynamic names or paths as arguments.

After changing catalogs, validate and regenerate with MyGo:

```sh
go run github.com/egoist/mygo/cmd/mygo i18n check
go run github.com/egoist/mygo/cmd/mygo generate
go test ./locales
```

Commit the generated `catalog_gen.go` so ordinary Go builds remain independent
of the generator. Regional date and time formats use MyGo's CLDR formatting.

For layout review, MyGo's `MYGO_LOCALE=en-XA` pseudo locale expands messages,
and `MYGO_LOCALE=ar-XB` exercises right-to-left layout. Check long labels,
window controls, dialogs and scroll gutters in a real native window.
