package menuet

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SnapshotSchema is the version string embedded in every snapshot file.
// Bump this when the snapshot JSON shape changes so consumers can detect
// incompatible versions. Existing fields should never change meaning.
const SnapshotSchema = "menuet-snapshot/v1"

// snapshotMaxDepth caps recursion into submenus when building a snapshot.
// A malformed Children callback that always returns more children would
// otherwise hang the snapshot. Eight levels is far past anything sensible
// in a menu.
const snapshotMaxDepth = 8

// Snapshot is a JSON-serializable picture of an App's current menu —
// MenuState plus the resolved top-level items, with submenus recursively
// expanded. It's produced by `MENUET_SNAPSHOT_PATH=… ./app` and consumed
// by the HTML renderer to draw a faithful mockup outside AppKit (for
// websites, README screenshots, regression tests).
//
// The snapshot is pure data — there is no code path back to the app from
// a snapshot file, so rendering one is safe even if the file came from a
// third party. The renderer's job is to honor the data and reject any
// shape outside the schema.
type Snapshot struct {
	Schema string         `json:"schema"`
	State  *MenuState     `json:"state,omitempty"`
	Items  []SnapshotItem `json:"items"`
}

// SnapshotItem is the snapshot-only mirror of MenuItem. It is intentionally
// separate from internalItem (which carries cgo bookkeeping fields like
// Unique/ParentUnique and a live *MenuItem pointer) so the snapshot stays
// a self-contained data payload with no live references.
type SnapshotItem struct {
	// Type is "regular", "separator", or "search". Empty means regular for
	// backward-compatibility when the field is omitted.
	Type string `json:"type,omitempty"`

	Text       string     `json:"text,omitempty"`
	Runs       []TextRun  `json:"runs,omitempty"`
	Subtitle   []TextRun  `json:"subtitle,omitempty"`
	Image      string     `json:"image,omitempty"`
	FontSize   int        `json:"fontSize,omitempty"`
	FontWeight FontWeight `json:"fontWeight,omitempty"`
	Color      Color      `json:"color,omitempty"`
	Monospaced bool       `json:"monospaced,omitempty"`
	Shortcut   *Shortcut  `json:"shortcut,omitempty"`
	State      bool       `json:"state,omitempty"`

	// Children are the expanded submenu, populated by the snapshotter.
	Children []SnapshotItem `json:"children,omitempty"`
}

// maybeWriteSnapshot is the entry point used by RunApplication. If
// MENUET_SNAPSHOT_PATH is unset, it returns false and normal startup
// continues. If set, it waits MENUET_SNAPSHOT_DELAY (default 2s) so the
// app's startup goroutines have a chance to populate state, then writes
// the snapshot to the path and returns true. The caller is expected to
// short-circuit further startup (no AppKit, no Carbon, no goroutines)
// when it returns true.
//
// The delay is the one tunable: an app that fetches data on startup will
// often want a longer delay before its menu reflects real values. Set
// MENUET_SNAPSHOT_DELAY=5s (or any time.Duration string) to override.
func (a *Application) maybeWriteSnapshot() bool {
	path := os.Getenv("MENUET_SNAPSHOT_PATH")
	if path == "" {
		return false
	}
	delay := 2 * time.Second
	if s := os.Getenv("MENUET_SNAPSHOT_DELAY"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "menuet: invalid MENUET_SNAPSHOT_DELAY %q: %v\n", s, err)
			os.Exit(2)
		}
		delay = d
	}
	time.Sleep(delay)
	if err := a.writeSnapshot(path); err != nil {
		fmt.Fprintf(os.Stderr, "menuet: snapshot write failed: %v\n", err)
		os.Exit(1)
	}
	return true
}

// writeSnapshot captures the current MenuState and top-level Children,
// recursively expands submenus, and writes the result as indented JSON.
func (a *Application) writeSnapshot(path string) error {
	snap := Snapshot{
		Schema: SnapshotSchema,
		State:  a.currentState,
	}
	if a.Children != nil {
		snap.Items = snapshotItems(a.Children(), 0)
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// snapshotItems converts a []MenuItem to []SnapshotItem, recursing into
// each item's Children callback up to snapshotMaxDepth. It deliberately
// does NOT share code with buildInternalItem because that path has cgo
// side effects (registerGlobalHotkey) that must not fire during a
// snapshot.
func snapshotItems(items []MenuItem, depth int) []SnapshotItem {
	if depth >= snapshotMaxDepth {
		return nil
	}
	out := make([]SnapshotItem, len(items))
	for i, item := range items {
		out[i] = snapshotItem(item, depth)
	}
	return out
}

func snapshotItem(item MenuItem, depth int) SnapshotItem {
	switch v := item.(type) {
	case Regular:
		s := SnapshotItem{
			Type:       "regular",
			Text:       v.Text,
			Runs:       v.Runs,
			Subtitle:   v.Subtitle,
			Image:      v.Image,
			FontSize:   v.FontSize,
			FontWeight: v.FontWeight,
			Color:      v.Color,
			Monospaced: v.Monospaced,
			Shortcut:   v.Shortcut,
			State:      v.State,
		}
		if v.Children != nil {
			s.Children = snapshotItems(v.Children(), depth+1)
		}
		return s
	case Separator:
		return SnapshotItem{Type: "separator"}
	case Search:
		s := SnapshotItem{Type: "search", Text: v.Placeholder}
		// Snapshot search results with an empty query — the same call the
		// real menu makes when the search field first appears. Authors who
		// want a richer demo can synthesize results conditionally on the
		// MENUET_SNAPSHOT_PATH env var inside their Results callback.
		if v.Results != nil {
			s.Children = snapshotItems(v.Results(""), depth+1)
		}
		return s
	}
	return SnapshotItem{}
}
