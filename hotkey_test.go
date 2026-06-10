package menuet

import (
	"sync/atomic"
	"testing"
)

func resetHotkeyRegistry() {
	hotkeyMu.Lock()
	hotkeyCallback = map[uint32]func(){}
	hotkeyByTuple = map[Shortcut]uint32{}
	hotkeyNextID = 0
	hotkeyMu.Unlock()
}

func TestShortcutZero(t *testing.T) {
	if !(Shortcut{}).IsZero() {
		t.Error("Shortcut{} should be zero")
	}
	if (Shortcut{KeyCode: KeyN}).IsZero() {
		t.Error("Shortcut with KeyCode set should NOT be zero")
	}
	if (Shortcut{Modifiers: ModCmd}).IsZero() {
		t.Error("Shortcut with Modifiers set should NOT be zero")
	}
}

func TestRegisterGlobalHotkeyAssignsID(t *testing.T) {
	resetHotkeyRegistry()
	defer resetHotkeyRegistry()
	id := registerGlobalHotkey(
		Shortcut{KeyCode: KeyN, Modifiers: ModCmd | ModShift},
		func() {},
	)
	if id == 0 {
		t.Fatal("expected non-zero id on successful registration")
	}
}

func TestRegisterGlobalHotkeyDedupesIdenticalShortcut(t *testing.T) {
	resetHotkeyRegistry()
	defer resetHotkeyRegistry()
	sc := Shortcut{KeyCode: KeyN, Modifiers: ModCmd}
	id1 := registerGlobalHotkey(sc, func() {})
	id2 := registerGlobalHotkey(sc, func() {}) // second action should be ignored
	if id1 != id2 {
		t.Errorf("expected same id for duplicate shortcut, got id1=%d id2=%d", id1, id2)
	}
}

func TestRegisterGlobalHotkeyIgnoresZero(t *testing.T) {
	resetHotkeyRegistry()
	defer resetHotkeyRegistry()
	id := registerGlobalHotkey(Shortcut{}, func() {})
	if id != 0 {
		t.Errorf("zero shortcut should return id=0, got %d", id)
	}
}

func TestRegisterGlobalHotkeyIgnoresNilAction(t *testing.T) {
	resetHotkeyRegistry()
	defer resetHotkeyRegistry()
	id := registerGlobalHotkey(Shortcut{KeyCode: KeyN, Modifiers: ModCmd}, nil)
	if id != 0 {
		t.Errorf("nil action should return id=0, got %d", id)
	}
}

func TestHotkeyFiredInvokesCallback(t *testing.T) {
	resetHotkeyRegistry()
	defer resetHotkeyRegistry()
	var fired int32
	id := registerGlobalHotkey(
		Shortcut{KeyCode: KeySpace, Modifiers: ModCmd | ModAlt},
		func() { atomic.StoreInt32(&fired, 1) },
	)
	hotkeyMu.RLock()
	cb := hotkeyCallback[id]
	hotkeyMu.RUnlock()
	if cb == nil {
		t.Fatal("callback not stored")
	}
	cb()
	if atomic.LoadInt32(&fired) != 1 {
		t.Errorf("callback didn't fire")
	}
}
