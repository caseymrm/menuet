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
type Regular struct {
	Text       string
	Image      string // In Resources dir or URL, should have height 16
	FontSize   int    // Default: 14
	FontWeight FontWeight
	State      bool // shows checkmark when set

	Clicked  func()
	Children func() []MenuItem
}

func (Regular) menuItem() {}

// Separator is a horizontal divider between menu rows.
type Separator struct{}

func (Separator) menuItem() {}

type internalItem struct {
	Unique       string
	ParentUnique string

	// Fields serialized to the ObjC side. The shape is intentionally flat
	// so the existing populate: in menuet.m doesn't need to know about
	// MenuItem types — it just reads Type and the matching fields.
	Type        string
	Text        string
	Image       string
	FontSize    int
	FontWeight  FontWeight
	State       bool
	HasChildren bool
	Clickable   bool

	// item is the original MenuItem, kept so click and Children callbacks
	// can be looked up later when ObjC re-enters via itemClicked / children.
	item MenuItem
}

func buildInternalItem(item MenuItem, unique, parentUnique string) internalItem {
	out := internalItem{Unique: unique, ParentUnique: parentUnique, item: item}
	switch v := item.(type) {
	case Regular:
		out.Text = v.Text
		out.Image = v.Image
		out.FontSize = v.FontSize
		out.FontWeight = v.FontWeight
		out.State = v.State
		out.Clickable = v.Clicked != nil
		out.HasChildren = v.Children != nil
	case Separator:
		out.Type = "separator"
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
