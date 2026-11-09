package menuet

import "C"

// MenuBarIsDark reports whether the menu bar currently has a dark
// appearance. The value follows the menu bar itself, not the system setting:
// with a dark desktop picture the bar can be dark even in light system mode.
// It is meant for applications that set MenuState.ColorImage and want to
// pick a different image per bar appearance. Before RunApplication has
// started the app, and until the first report, it returns false.
func (a *Application) MenuBarIsDark() bool {
	a.appearanceMutex.Lock()
	defer a.appearanceMutex.Unlock()
	return a.menuBarDark
}

// setMenuBarDark stores the reported appearance and queues a
// MenuBarAppearanceChanged call on the first report and whenever the value
// changes. Calls run on a separate goroutine, one at a time and in order.
func (a *Application) setMenuBarDark(dark bool) {
	a.appearanceMutex.Lock()
	changed := !a.menuBarReported || a.menuBarDark != dark
	a.menuBarReported = true
	a.menuBarDark = dark
	callback := a.MenuBarAppearanceChanged
	if !changed || callback == nil {
		a.appearanceMutex.Unlock()
		return
	}
	a.appearanceQueue = append(a.appearanceQueue, dark)
	start := !a.appearanceDraining
	a.appearanceDraining = true
	a.appearanceMutex.Unlock()
	if start {
		go a.drainAppearanceQueue(callback)
	}
}

func (a *Application) drainAppearanceQueue(callback func(bool)) {
	for {
		a.appearanceMutex.Lock()
		if len(a.appearanceQueue) == 0 {
			a.appearanceDraining = false
			a.appearanceMutex.Unlock()
			return
		}
		dark := a.appearanceQueue[0]
		a.appearanceQueue = a.appearanceQueue[1:]
		a.appearanceMutex.Unlock()
		callback(dark)
	}
}

//export menuBarAppearanceChanged
func menuBarAppearanceChanged(dark bool) {
	App().setMenuBarDark(dark)
}
