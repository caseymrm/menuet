package menuet

import (
	"testing"
	"time"
)

func TestMenuBarAppearance(t *testing.T) {
	a := &Application{}
	got := make(chan bool, 4)
	a.MenuBarAppearanceChanged = func(dark bool) { got <- dark }

	if a.MenuBarIsDark() {
		t.Fatal("zero value should be light")
	}
	expect := func(want bool) {
		t.Helper()
		select {
		case v := <-got:
			if v != want {
				t.Fatalf("callback got %v, want %v", v, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("callback for %v did not fire", want)
		}
	}
	expectNone := func() {
		t.Helper()
		select {
		case v := <-got:
			t.Fatalf("unexpected callback %v", v)
		case <-time.After(50 * time.Millisecond):
		}
	}

	a.setMenuBarDark(false) // first report fires even when light
	expect(false)
	a.setMenuBarDark(false)
	expectNone()
	a.setMenuBarDark(true)
	expect(true)
	if !a.MenuBarIsDark() {
		t.Error("getter should be dark")
	}
	a.setMenuBarDark(true)
	expectNone()
	a.setMenuBarDark(false)
	expect(false)
	if a.MenuBarIsDark() {
		t.Error("getter should be light")
	}
}
