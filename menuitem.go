package menuet

import (
	"log"
	"strings"
	"time"

	"github.com/caseymrm/askm"
)

// ItemType represents what type of menu item this is
type ItemType string

const (
	// Regular is a normal item with text and optional callback
	Regular ItemType = ""
	// Separator is a horizontal line
	Separator = "separator"
	// Root is the top level menu directly off the menubar
	Root = "root"
	// TODO: StartAtLogin, Quit, Image, Spinner, etc
)

// MenuItem represents one item in the dropdown
type MenuItem struct {
	Type ItemType
	Data interface{}

	Text       string
	FontSize   int // Default: 14
	FontWeight FontWeight
	State      bool // shows checkmark when set
	Disabled   bool
	Children   bool

	// If set, the application's Clicked is not called for this item
	Clicked func() `json:"-"`
	// If set, the application's MenuOpened is not called for this item
	MenuOpened func() []MenuItem `json:"-"`
}

type internalItem struct {
	Unique       string
	ParentUnique string

	MenuItem
}

func (a *Application) menuOpened(unique string) []internalItem {
	a.visibleMenuItemsMutex.RLock()
	item, ok := a.visibleMenuItems[unique]
	a.visibleMenuItemsMutex.RUnlock()
	if !ok {
		if strings.HasSuffix(unique, ":root") {
			// Fill in this synthetic item
			item.Unique = unique
			item.Type = Root
		} else {
			log.Printf("Item not found for menuOpened: %s", unique)
		}
	}
	var items []MenuItem
	if item.MenuOpened != nil {
		items = item.MenuOpened()
	} else {
		if a.MenuOpened == nil {
			return nil
		}
		items = a.MenuOpened(item.MenuItem)
	}
	internalItems := make([]internalItem, len(items))
	for ind, item := range items {
		a.visibleMenuItemsMutex.Lock()
		newUnique := askm.ArbitraryKeyNotInMap(a.visibleMenuItems)
		internal := internalItem{
			Unique:       newUnique,
			ParentUnique: unique,
			MenuItem:     item,
		}
		if internal.MenuOpened != nil {
			internal.Children = true
		}
		a.visibleMenuItems[newUnique] = internal
		internalItems[ind] = internal
		a.visibleMenuItemsMutex.Unlock()
	}
	return internalItems
}

func (a *Application) menuClosed(unique string) {
	go func() {
		// We receive menuClosed before clicked, so wait a second before discarding the data just in case
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
