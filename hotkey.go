package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon

#import <Cocoa/Cocoa.h>

void registerHotkey(uint32_t id, uint32_t keyCode, uint32_t modifiers);
void unregisterHotkey(uint32_t id);
*/
import "C"
import (
	"log"
	"sync"
)

// ModifierMask is a bitmask of keyboard modifier keys for a Shortcut.
// OR the constants together: ModCmd|ModShift.
type ModifierMask uint32

// Modifier flags. Match the values Carbon's RegisterEventHotKey expects.
const (
	ModCmd   ModifierMask = 1 << 8  // cmdKey
	ModShift ModifierMask = 1 << 9  // shiftKey
	ModAlt   ModifierMask = 1 << 11 // optionKey
	ModCtrl  ModifierMask = 1 << 12 // controlKey
)

// Shortcut is a global keyboard shortcut. When set on a Regular menu item,
// the shortcut both:
//
//   - displays in the menu like a standard Apple shortcut (⌘N etc.), and
//   - fires the item's Clicked callback when pressed system-wide, even
//     when this app isn't frontmost
//
// Identical (KeyCode, Modifiers) tuples across multiple menu items are
// not allowed — only the first registered wins; subsequent ones are
// silently ignored. Pick distinct shortcuts per action.
type Shortcut struct {
	KeyCode   uint16
	Modifiers ModifierMask
}

// IsZero reports whether s is the zero value (no key and no modifiers).
func (s Shortcut) IsZero() bool { return s.KeyCode == 0 && s.Modifiers == 0 }

// macOS virtual key codes. Lowercase letters in QWERTY layout. Not
// exhaustive — add as needed.
const (
	KeyA = 0
	KeyB = 11
	KeyC = 8
	KeyD = 2
	KeyE = 14
	KeyF = 3
	KeyG = 5
	KeyH = 4
	KeyI = 34
	KeyJ = 38
	KeyK = 40
	KeyL = 37
	KeyM = 46
	KeyN = 45
	KeyO = 31
	KeyP = 35
	KeyQ = 12
	KeyR = 15
	KeyS = 1
	KeyT = 17
	KeyU = 32
	KeyV = 9
	KeyW = 13
	KeyX = 7
	KeyY = 16
	KeyZ = 6

	Key0 = 29
	Key1 = 18
	Key2 = 19
	Key3 = 20
	Key4 = 21
	Key5 = 23
	Key6 = 22
	Key7 = 26
	Key8 = 28
	Key9 = 25

	KeySpace  = 49
	KeyReturn = 36
	KeyTab    = 48
	KeyEsc    = 53

	KeyF1  = 122
	KeyF2  = 120
	KeyF3  = 99
	KeyF4  = 118
	KeyF5  = 96
	KeyF6  = 97
	KeyF7  = 98
	KeyF8  = 100
	KeyF9  = 101
	KeyF10 = 109
	KeyF11 = 103
	KeyF12 = 111

	KeyLeft  = 123
	KeyRight = 124
	KeyDown  = 125
	KeyUp    = 126
)

// hotkeyRegistry tracks active global hotkeys so the cgo callback can
// route incoming triggers to the right Go callback. Keyed by the uint32
// ID we hand to Carbon at registration time.
var (
	hotkeyMu       sync.RWMutex
	hotkeyCallback = map[uint32]func(){}
	hotkeyByTuple  = map[Shortcut]uint32{} // dedup identical shortcuts
	hotkeyNextID   uint32                  // monotonic; never reuse
)

// registerHotkey assigns an ID for s and registers a Carbon hotkey that
// invokes action when fired. If s is already registered, the second
// registration is ignored (only the first action wins) and the original
// ID is returned. Returns 0 if registration fails (e.g. duplicate).
func registerGlobalHotkey(s Shortcut, action func()) uint32 {
	if s.IsZero() || action == nil {
		return 0
	}
	hotkeyMu.Lock()
	if id, ok := hotkeyByTuple[s]; ok {
		hotkeyMu.Unlock()
		return id
	}
	hotkeyNextID++
	id := hotkeyNextID
	hotkeyCallback[id] = action
	hotkeyByTuple[s] = id
	hotkeyMu.Unlock()
	C.registerHotkey(C.uint32_t(id), C.uint32_t(s.KeyCode), C.uint32_t(s.Modifiers))
	return id
}

//export hotkeyFired
func hotkeyFired(id C.uint32_t) {
	hotkeyMu.RLock()
	cb := hotkeyCallback[uint32(id)]
	hotkeyMu.RUnlock()
	if cb == nil {
		log.Printf("menuet: hotkey id=%d fired with no callback registered", id)
		return
	}
	go cb()
}
