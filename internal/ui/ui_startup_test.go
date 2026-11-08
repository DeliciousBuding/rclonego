package desktop

import "testing"

func TestIndependentProfileCannotRegisterDefaultProfileAtLogin(t *testing.T) {
	model := New(testApp(t), "")
	if model.canConfigureLogin() {
		t.Fatal("an independent profile could register startup without its profile arguments")
	}
}
