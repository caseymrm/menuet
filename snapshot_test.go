package menuet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotItemsBasicShape(t *testing.T) {
	items := []MenuItem{
		Regular{Text: "Refresh", Shortcut: &Shortcut{KeyCode: 15, Modifiers: ModCmd}},
		Separator{},
		Regular{Text: "Notify on scoring plays", State: true},
	}
	got := snapshotItems(items, 0)
	if len(got) != 3 {
		t.Fatalf("want 3 items, got %d", len(got))
	}
	if got[0].Type != "regular" || got[0].Text != "Refresh" {
		t.Errorf("first item: %+v", got[0])
	}
	if got[0].Shortcut == nil || got[0].Shortcut.Modifiers != ModCmd {
		t.Errorf("shortcut lost: %+v", got[0])
	}
	if got[1].Type != "separator" {
		t.Errorf("separator: %+v", got[1])
	}
	if !got[2].State {
		t.Errorf("State not captured: %+v", got[2])
	}
}

func TestSnapshotRecursesIntoChildren(t *testing.T) {
	items := []MenuItem{
		Regular{
			Text: "Favorites",
			Children: func() []MenuItem {
				return []MenuItem{Regular{Text: "Lakers"}}
			},
		},
	}
	got := snapshotItems(items, 0)
	if len(got[0].Children) != 1 || got[0].Children[0].Text != "Lakers" {
		t.Errorf("submenu not captured: %+v", got)
	}
}

func TestSnapshotCapsDepth(t *testing.T) {
	// A pathological recursive Children that always returns one more level.
	var build func(depth int) MenuItem
	build = func(depth int) MenuItem {
		return Regular{
			Text: "x",
			Children: func() []MenuItem {
				return []MenuItem{build(depth + 1)}
			},
		}
	}
	got := snapshotItems([]MenuItem{build(0)}, 0)
	depth := 0
	cur := got[0]
	for len(cur.Children) > 0 {
		depth++
		cur = cur.Children[0]
		if depth > snapshotMaxDepth+2 {
			t.Fatalf("recursion not capped: still going at depth %d", depth)
		}
	}
}

func TestSnapshotDoesNotRegisterHotkey(t *testing.T) {
	// buildInternalItem registers a Carbon hotkey when a Regular has both
	// a Shortcut and a Clicked callback. snapshotItem must NOT — calling
	// registerGlobalHotkey from a CLI snapshot would crash (no AppKit run
	// loop) or leak Carbon resources into a soon-to-exit process. This
	// test simply exercises the path and relies on the absence of any cgo
	// call (the test runs without an AppKit loop).
	items := []MenuItem{Regular{
		Text:     "Refresh",
		Shortcut: &Shortcut{KeyCode: 15, Modifiers: ModCmd},
		Clicked:  func() {},
	}}
	got := snapshotItems(items, 0)
	if got[0].Shortcut == nil {
		t.Errorf("shortcut lost: %+v", got[0])
	}
}

func TestWriteSnapshotProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.json")
	a := &Application{
		currentState: &MenuState{Title: "Hello"},
		Children: func() []MenuItem {
			return []MenuItem{Regular{Text: "Refresh"}}
		},
	}
	if err := a.writeSnapshot(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}
	if snap.Schema != SnapshotSchema {
		t.Errorf("schema: got %q", snap.Schema)
	}
	if snap.State == nil || snap.State.Title != "Hello" {
		t.Errorf("state: %+v", snap.State)
	}
	if len(snap.Items) != 1 || snap.Items[0].Text != "Refresh" {
		t.Errorf("items: %+v", snap.Items)
	}
}
