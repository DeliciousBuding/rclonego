//go:build darwin

package desktop

import (
	"bytes"
	"sync"

	"github.com/ebitengine/purego"
)

var languagePreferences struct {
	once             sync.Once
	available        bool
	mainBundle       func() uintptr
	bundleIdentifier func(uintptr) uintptr
	newString        func(uintptr, string, uint32) uintptr
	copyValue        func(uintptr, uintptr, uintptr, uintptr) uintptr
	synchronize      func(uintptr, uintptr, uintptr) bool
	currentUser      uintptr
	anyHost          uintptr
	typeID           func(uintptr) uintptr
	arrayTypeID      func() uintptr
	stringTypeID     func() uintptr
	arrayCount       func(uintptr) int64
	arrayValue       func(uintptr, int64) uintptr
	stringCString    func(uintptr, *byte, int64, uint32) bool
	release          func(uintptr)
}

// macOS keeps MyGo's override as the application's AppleLanguages preference,
// which is also editable in System Settings. Read that same preference through
// CoreFoundation instead of creating another language file.
func platformLanguagePreference(string) string {
	languagePreferences.once.Do(func() {
		library, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&languagePreferences.mainBundle, library, "CFBundleGetMainBundle")
		purego.RegisterLibFunc(&languagePreferences.bundleIdentifier, library, "CFBundleGetIdentifier")
		purego.RegisterLibFunc(&languagePreferences.newString, library, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&languagePreferences.copyValue, library, "CFPreferencesCopyValue")
		purego.RegisterLibFunc(&languagePreferences.synchronize, library, "CFPreferencesSynchronize")
		user, userErr := purego.Dlsym(library, "kCFPreferencesCurrentUser")
		host, hostErr := purego.Dlsym(library, "kCFPreferencesAnyHost")
		if userErr != nil || hostErr != nil || user == 0 || host == 0 {
			return
		}
		// CoreFoundation exports these immutable CFStringRef constants as
		// pointer-valued symbols, the same way MyGo reads the preference domain.
		var copyPointer func(*uintptr, uintptr, uintptr) uintptr
		purego.RegisterLibFunc(&copyPointer, library, "memcpy")
		copyPointer(&languagePreferences.currentUser, user, 8)
		copyPointer(&languagePreferences.anyHost, host, 8)
		purego.RegisterLibFunc(&languagePreferences.typeID, library, "CFGetTypeID")
		purego.RegisterLibFunc(&languagePreferences.arrayTypeID, library, "CFArrayGetTypeID")
		purego.RegisterLibFunc(&languagePreferences.stringTypeID, library, "CFStringGetTypeID")
		purego.RegisterLibFunc(&languagePreferences.arrayCount, library, "CFArrayGetCount")
		purego.RegisterLibFunc(&languagePreferences.arrayValue, library, "CFArrayGetValueAtIndex")
		purego.RegisterLibFunc(&languagePreferences.stringCString, library, "CFStringGetCString")
		purego.RegisterLibFunc(&languagePreferences.release, library, "CFRelease")
		languagePreferences.available = true
	})
	if !languagePreferences.available {
		return ""
	}
	bundle := languagePreferences.mainBundle()
	if bundle == 0 {
		return ""
	}
	identifier := languagePreferences.bundleIdentifier(bundle)
	if identifier == 0 {
		return ""
	}
	const utf8 = 0x08000100
	key := languagePreferences.newString(0, "AppleLanguages", utf8)
	defer languagePreferences.release(key)
	languagePreferences.synchronize(identifier, languagePreferences.currentUser, languagePreferences.anyHost)
	// CopyAppValue searches global defaults too. Read only this application's
	// domain so global AppleLanguages never looks like a manual app override.
	value := languagePreferences.copyValue(key, identifier, languagePreferences.currentUser, languagePreferences.anyHost)
	if value == 0 {
		return ""
	}
	defer languagePreferences.release(value)
	if languagePreferences.typeID(value) != languagePreferences.arrayTypeID() || languagePreferences.arrayCount(value) == 0 {
		return ""
	}
	tag := languagePreferences.arrayValue(value, 0)
	if tag == 0 || languagePreferences.typeID(tag) != languagePreferences.stringTypeID() {
		return ""
	}
	buffer := make([]byte, 256)
	if !languagePreferences.stringCString(tag, &buffer[0], int64(len(buffer)), utf8) {
		return ""
	}
	end := bytes.IndexByte(buffer, 0)
	if end < 0 {
		return ""
	}
	return string(buffer[:end])
}
