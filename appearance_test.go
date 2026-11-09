package menuet

import (
	"testing"
	"time"
)

func TestMenuBarAppearance(t *testing.T) {
	old := appearanceSettleDelay
	appearanceSettleDelay = 20 * time.Millisecond
	defer func() { appearanceSettleDelay = old }()

	a := &Application{}
	got := make(chan bool, 8)
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
		if a.MenuBarIsDark() != want {
			t.Errorf("getter = %v, want %v", a.MenuBarIsDark(), want)
		}
	}
	expectNone := func() {
		t.Helper()
		select {
		case v := <-got:
			t.Fatalf("unexpected callback %v", v)
		case <-time.After(100 * time.Millisecond):
		}
	}

	a.setMenuBarDark(false) // first settled value fires even when light
	expect(false)
	a.setMenuBarDark(false)
	expectNone()

	// A quick flip sequence yields exactly one call with the final value.
	for _, v := range []bool{false, true, false, true} {
		a.setMenuBarDark(v)
	}
	expect(true)
	expectNone()

	// Flipping back to the already reported value does not fire.
	a.setMenuBarDark(false)
	a.setMenuBarDark(true)
	expectNone()

	a.setMenuBarDark(false)
	expect(false)
}
