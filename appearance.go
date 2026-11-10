package menuet

import "C"

import "time"

// MenuBarIsDark reports whether the menu bar currently has a dark
// appearance. The value follows the menu bar itself, not the system setting:
// with a dark desktop picture the bar can be dark even in light system mode.
// It is meant for applications that set MenuState.ColorImage and want to
// pick a different image per bar appearance. Before RunApplication has
// started the app, and until the first value has settled, it returns false.
// Short flips while the status item is first shown are merged, so the value
// lags the bar by about 150 ms.
func (a *Application) MenuBarIsDark() bool {
	a.appearanceMutex.Lock()
	defer a.appearanceMutex.Unlock()
	return a.menuBarDark
}

// appearanceSettleDelay is how long the reported appearance must stay
// unchanged before it takes effect. AppKit flips the status button's
// appearance several times while the item is first shown.
var appearanceSettleDelay = 150 * time.Millisecond

// setMenuBarDark records a reported appearance. It takes effect (getter and
// MenuBarAppearanceChanged) once no further report arrived for
// appearanceSettleDelay; the callback fires for the first settled value and
// whenever the settled value differs from the last one reported.
func (a *Application) setMenuBarDark(dark bool) {
	a.appearanceMutex.Lock()
	defer a.appearanceMutex.Unlock()
	a.appearanceGeneration++
	generation := a.appearanceGeneration
	if a.appearanceTimer != nil {
		a.appearanceTimer.Stop()
	}
	a.appearanceTimer = time.AfterFunc(appearanceSettleDelay, func() {
		a.settleMenuBarDark(dark, generation)
	})
}

func (a *Application) settleMenuBarDark(dark bool, generation uint64) {
	a.appearanceMutex.Lock()
	if generation != a.appearanceGeneration {
		a.appearanceMutex.Unlock()
		return
	}
	changed := !a.menuBarReported || a.menuBarDark != dark
	a.menuBarReported = true
	a.menuBarDark = dark
	callback := a.MenuBarAppearanceChanged
	a.appearanceMutex.Unlock()
	if changed && callback != nil {
		callback(dark)
	}
}

//export menuBarAppearanceChanged
func menuBarAppearanceChanged(dark bool) {
	App().setMenuBarDark(dark)
}
