package menuet

import (
	"log"
	"strings"
	"time"

	"github.com/caseymrm/askm"
)

// MenuItem is the marker interface for items in a menu.
// The concrete types in this package implement it: Regular and Separator.
// A future Search type for in-menu search fields will implement it too.
type MenuItem interface {
	menuItem()
}

// Regular is a standard menu row. Set Text and optionally Clicked
// (callback when activated) and Children (returns a submenu).
//
// For mixed styling within a single row — e.g. "Status: FAILED" where
// FAILED is red and bold — set Runs to a slice of TextRun. Runs takes
// precedence over Text when non-empty.
type Regular struct {
	Text       string
	Runs       []TextRun // when non-empty, overrides Text
	Image      string    // In Resources dir or URL, should have height 16
	FontSize   int       // Default: 14
	FontWeight FontWeight
	Color      Color // zero = system default
	Monospaced bool
	State      bool // shows checkmark when set

	// Shortcut, when non-nil, displays in the menu like a standard Apple
	// shortcut (⌘N etc.) AND registers a system-wide hotkey: pressing the
	// key combination triggers Clicked even when this app isn't frontmost.
	// Identical Shortcuts across multiple Regular items: only the first
	// wins; subsequent registrations are silently ignored.
	Shortcut *Shortcut

	// Subtitle, when non-empty, renders as a dimmer second line below the
	// main title — Apple's native NSMenuItem.subtitle on macOS 14+. The
	// runs' per-segment colors aren't honored on this path (NSMenuItem.subtitle
	// is a plain string property); only the concatenated text appears in
	// the system's standard subtitle styling.
	Subtitle []TextRun

	Clicked  func()
	Children func() []MenuItem
}

func (Regular) menuItem() {}

// Separator is a horizontal divider between menu rows.
type Separator struct{}

func (Separator) menuItem() {}

// Search is an in-menu search field. The Results callback fires on every
// keystroke with the current query (empty string when the menu first
// opens). Returned items appear immediately below the search field and
// replace whatever was there before. The last query is remembered across
// menu opens.
//
// Press Enter to activate the top result, Esc to dismiss, or click any
// result. Arrow keys do not navigate result items — NSMenu's tracking
// loop owns them at a layer outside this library's reach.
//
// Apps that use Search cannot be distributed via the Mac App Store —
// the implementation uses a private NSPopupMenuWindow selector
// (setKeyOverride:) to engage the text field's input context during
// menu tracking. Stable on current macOS but private-API-rejection
// material for the App Store. Direct-distribution apps are unaffected.
type Search struct {
	Placeholder string
	Results     func(query string) []MenuItem
}

func (Search) menuItem() {}

// DefaultMaxImageWidth and DefaultMaxImageHeight bound an Image whose
// MaxWidth / MaxHeight are left at zero, so an unscaled screenshot can't
// push the menu off the screen.
const (
	DefaultMaxImageWidth  = 480
	DefaultMaxImageHeight = 360
)

// Image is a menu row that renders a picture in place of text. As the only
// child of a Regular it makes a submenu that *is* an image; alongside other
// items it's a picture above a caption, a "Open full size" row, and so on:
//
//	menuet.Regular{
//	    Text: "Living room iMac",
//	    Children: func() []menuet.MenuItem {
//	        return []menuet.MenuItem{
//	            menuet.Image{Data: screenshotPNG, MaxWidth: 480},
//	            menuet.Separator{},
//	            menuet.Regular{Text: "Open full size", Clicked: openFull},
//	        }
//	    },
//	}
//
// This is a different thing from Regular.Image, which is a small (16pt-tall)
// icon drawn to the *left* of a row's text. Image is the row.
//
// Exactly one of Data or Path must be set. menuet never fetches an image
// itself: there is deliberately no URL field, because the fetch would land on
// the thread building the menu (hanging it) and could not carry an
// Authorization header. Fetch it in Go — where you already have your auth,
// caching and retries — and hand over the bytes or a path.
type Image struct {
	// Data is the encoded image (PNG, JPEG, …). Use it when your Go code
	// already holds the bytes: a screenshot pulled from an authenticated API,
	// a chart you rendered in-process. The bytes cross the cgo bridge on every
	// menu open, so prefer Path for large, unchanging images.
	Data []byte

	// Path is an absolute path to an image file. Cheaper than Data for large
	// images — only the path crosses the bridge — and cached by path, size and
	// modification time, so rewriting the file shows the new picture.
	Path string

	// MaxWidth and MaxHeight bound the rendered picture. It is scaled down to
	// fit inside them, preserving aspect ratio, and is never scaled up. A zero
	// value means DefaultMaxImageWidth / DefaultMaxImageHeight for that axis.
	MaxWidth  int
	MaxHeight int

	// Clicked, when set, makes the row respond to clicks and highlight under
	// the pointer, like a Regular. Leave nil for a purely decorative picture.
	Clicked func()
}

func (Image) menuItem() {}

type internalItem struct {
	Unique       string
	ParentUnique string

	// Fields serialized to the ObjC side. The shape is intentionally flat
	// so the existing populate: in menuet.m doesn't need to know about
	// MenuItem types — it just reads Type and the matching fields.
	Type        string
	Text        string
	Runs        []TextRun `json:",omitempty"`
	Subtitle    []TextRun `json:",omitempty"`
	Image       string
	FontSize    int
	FontWeight  FontWeight
	Color       Color     `json:",omitempty"`
	Monospaced  bool      `json:",omitempty"`
	Shortcut    *Shortcut `json:",omitempty"`
	State       bool
	HasChildren bool
	Clickable   bool

	// Image ("image" Type) fields. ImageData is []byte, which encoding/json
	// base64-encodes for us; the ObjC side decodes it back.
	ImageData []byte `json:",omitempty"`
	ImagePath string `json:",omitempty"`
	MaxWidth  int    `json:",omitempty"`
	MaxHeight int    `json:",omitempty"`

	// item is the original MenuItem, kept so click and Children callbacks
	// can be looked up later when ObjC re-enters via itemClicked / children.
	item MenuItem
}

func buildInternalItem(item MenuItem, unique, parentUnique string) internalItem {
	out := internalItem{Unique: unique, ParentUnique: parentUnique, item: item}
	switch v := item.(type) {
	case Regular:
		out.Text = v.Text
		out.Runs = v.Runs
		out.Subtitle = v.Subtitle
		out.Image = v.Image
		out.FontSize = v.FontSize
		out.FontWeight = v.FontWeight
		out.Color = v.Color
		out.Monospaced = v.Monospaced
		out.Shortcut = v.Shortcut
		out.State = v.State
		out.Clickable = v.Clicked != nil
		out.HasChildren = v.Children != nil
		// Register the global hotkey, with the click callback as the
		// action. Duplicate registrations are deduped in hotkey.go.
		if v.Shortcut != nil && !v.Shortcut.IsZero() && v.Clicked != nil {
			registerGlobalHotkey(*v.Shortcut, v.Clicked)
		}
	case Separator:
		out.Type = "separator"
	case Search:
		out.Type = "search"
		// Text doubles as the placeholder string on the ObjC side.
		out.Text = v.Placeholder
	case Image:
		out.Type = "image"
		// Exactly one source. Neither means nothing to draw; both is
		// ambiguous. Say so rather than silently picking one — the row would
		// otherwise just render blank with no clue why.
		hasData, hasPath := len(v.Data) > 0, v.Path != ""
		switch {
		case !hasData && !hasPath:
			log.Printf("menuet: Image has neither Data nor Path; nothing to draw")
		case hasData && hasPath:
			log.Printf("menuet: Image has both Data and Path; set exactly one")
		}
		out.ImageData = v.Data
		out.ImagePath = v.Path
		// Substitute the default bounds here, once, so every consumer (the
		// ObjC bridge, snapshots) sees concrete values — no cross-language
		// duplication of the defaults.
		out.MaxWidth = v.MaxWidth
		if out.MaxWidth <= 0 {
			out.MaxWidth = DefaultMaxImageWidth
		}
		out.MaxHeight = v.MaxHeight
		if out.MaxHeight <= 0 {
			out.MaxHeight = DefaultMaxImageHeight
		}
		out.Clickable = v.Clicked != nil
	}
	return out
}

func (a *Application) children(unique string) []internalItem {
	a.visibleMenuItemsMutex.RLock()
	parent, ok := a.visibleMenuItems[unique]
	a.visibleMenuItemsMutex.RUnlock()

	var childrenFn func() []MenuItem
	if strings.HasSuffix(unique, ":root") {
		childrenFn = a.Children
	} else if ok {
		if reg, regOK := parent.item.(Regular); regOK {
			childrenFn = reg.Children
		}
	} else {
		log.Printf("Item not found for children: %s", unique)
	}
	if childrenFn == nil {
		return nil
	}

	items := childrenFn()
	internalItems := make([]internalItem, len(items))
	a.visibleMenuItemsMutex.Lock()
	defer a.visibleMenuItemsMutex.Unlock()
	for ind, item := range items {
		newUnique := askm.ArbitraryKeyNotInMap(a.visibleMenuItems)
		internal := buildInternalItem(item, newUnique, unique)
		a.visibleMenuItems[newUnique] = internal
		internalItems[ind] = internal
	}
	return internalItems
}

// searchResults runs the Search.Results callback for the search item with
// the given unique ID, replacing any previously-registered result items
// for this search and returning the fresh items to the ObjC side. Called
// on every keystroke and once with an empty query when the search field
// is first rendered.
func (a *Application) searchResults(searchUnique, query string) []internalItem {
	a.visibleMenuItemsMutex.RLock()
	item, ok := a.visibleMenuItems[searchUnique]
	a.visibleMenuItemsMutex.RUnlock()
	if !ok {
		log.Printf("Search item not found: %s", searchUnique)
		return nil
	}
	search, ok := item.item.(Search)
	if !ok {
		log.Printf("Item %s is not a Search: %T", searchUnique, item.item)
		return nil
	}
	if search.Results == nil {
		return nil
	}

	items := search.Results(query)

	a.visibleMenuItemsMutex.Lock()
	defer a.visibleMenuItemsMutex.Unlock()
	// Drop result items from the previous query — they were registered
	// with ParentUnique == searchUnique by an earlier call.
	for k, v := range a.visibleMenuItems {
		if v.ParentUnique == searchUnique {
			delete(a.visibleMenuItems, k)
		}
	}
	internalItems := make([]internalItem, len(items))
	for ind, child := range items {
		newUnique := askm.ArbitraryKeyNotInMap(a.visibleMenuItems)
		internal := buildInternalItem(child, newUnique, searchUnique)
		a.visibleMenuItems[newUnique] = internal
		internalItems[ind] = internal
	}
	return internalItems
}

func (a *Application) menuClosed(unique string) {
	go func() {
		// We receive menuClosed before clicked, so wait a moment before
		// discarding the data just in case a click is in flight.
		time.Sleep(100 * time.Millisecond)
		a.visibleMenuItemsMutex.Lock()
		for itemUnique, item := range a.visibleMenuItems {
			if item.ParentUnique == unique {
				delete(a.visibleMenuItems, itemUnique)
			}
		}
		a.visibleMenuItemsMutex.Unlock()
	}()
}
