package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef __MENUET_H_H__
#import "menuet.h"
#endif

*/
import "C"
import (
	"encoding/json"
	"log"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/caseymrm/askm"
)

// Application represents the OSX application
type Application struct {
	Name  string
	Label string

	// Clicked is called with the key string of a menu item that is selected, if the item doesn't have a Clicked function
	Clicked func(string)
	// MenuOpened is called to refresh menu items when clicked, empty string for the top level, skipped if the item has a MenuOpened
	MenuOpened func(string) []MenuItem

	// If Version and Repo are set, checks for updates every day
	AutoUpdate struct {
		Version string
		Repo    string // For example "caseymrm/menuet"
	}

	alertChannel          chan AlertClicked
	currentState          *MenuState
	nextState             *MenuState
	pendingStateChange    bool
	debounceMutex         sync.Mutex
	visibleMenuItemsMutex sync.RWMutex
	visibleMenuItems      map[string]internalItem
}

var appInstance *Application
var appOnce sync.Once

// App returns the application singleton
func App() *Application {
	appOnce.Do(func() {
		appInstance = &Application{
			visibleMenuItems: make(map[string]internalItem),
		}
	})
	return appInstance
}

// RunApplication does not return
func (a *Application) RunApplication() {
	if a.AutoUpdate.Version != "" && a.AutoUpdate.Repo != "" {
		go a.checkForUpdates()
	}
	C.createAndRunApplication()
}

// SetMenuState changes what is shown in the dropdown
func (a *Application) SetMenuState(state *MenuState) {
	if reflect.DeepEqual(a.currentState, state) {
		return
	}
	go a.sendState(state)
}

// MenuChanged refreshes any open menus
func (a *Application) MenuChanged() {
	C.menuChanged()
}

// MenuState represents the title and drop down,
type MenuState struct {
	Title string
	// This is the name of an image in the Resources directory
	Image string
}

func (a *Application) sendState(state *MenuState) {
	a.debounceMutex.Lock()
	a.nextState = state
	if a.pendingStateChange {
		a.debounceMutex.Unlock()
		return
	}
	a.pendingStateChange = true
	a.debounceMutex.Unlock()
	time.Sleep(100 * time.Millisecond)
	a.debounceMutex.Lock()
	a.pendingStateChange = false
	if reflect.DeepEqual(a.currentState, a.nextState) {
		a.debounceMutex.Unlock()
		return
	}
	a.currentState = a.nextState
	a.debounceMutex.Unlock()
	b, err := json.Marshal(a.currentState)
	if err != nil {
		log.Printf("Marshal: %v (%+v)", err, a.currentState)
		return
	}
	cstr := C.CString(string(b))
	C.setState(cstr)
	C.free(unsafe.Pointer(cstr))
}

func (a *Application) clicked(unique string) {
	a.visibleMenuItemsMutex.RLock()
	item, ok := a.visibleMenuItems[unique]
	a.visibleMenuItemsMutex.RUnlock()
	if !ok {
		log.Printf("Item not found for click: %s", unique)
	}
	if item.Clicked != nil {
		go item.Clicked()
		return
	}
	if a.Clicked == nil {
		return
	}
	go a.Clicked(item.Key)
}

func (a *Application) menuOpened(unique string) []internalItem {
	a.visibleMenuItemsMutex.RLock()
	item, ok := a.visibleMenuItems[unique]
	a.visibleMenuItemsMutex.RUnlock()
	if !ok && unique != "" {
		log.Printf("Item not found for menu open: %s", unique)
	}
	var items []MenuItem
	if item.MenuOpened != nil {
		items = item.MenuOpened()
	} else {
		if a.MenuOpened == nil {
			return nil
		}
		items = a.MenuOpened(item.Key)
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
	// menu close comes before the click, so delay removing the items for a sec (240ms highest observed)
	go func() {
		time.Sleep(2 * time.Second)
		a.visibleMenuItemsMutex.Lock()
		delete(a.visibleMenuItems, unique)
		for itemUnique, item := range a.visibleMenuItems {
			if item.ParentUnique == unique {
				delete(a.visibleMenuItems, itemUnique)
			}
		}
		a.visibleMenuItemsMutex.Unlock()
	}()
}

//export itemClicked
func itemClicked(uniqueCString *C.char) {
	unique := C.GoString(uniqueCString)
	App().clicked(unique)
}

//export menuOpened
func menuOpened(uniqueCString *C.char) *C.char {
	unique := C.GoString(uniqueCString)
	items := App().menuOpened(unique)
	if items == nil {
		return nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		log.Printf("Marshal: %v", err)
		return nil
	}
	return C.CString(string(b))
}

//export menuClosed
func menuClosed(uniqueCString *C.char) {
	unique := C.GoString(uniqueCString)
	App().menuClosed(unique)
}

//export runningAtStartup
func runningAtStartup() bool {
	return App().runningAtStartup()
}

//export toggleStartup
func toggleStartup() {
	a := App()
	if a.runningAtStartup() {
		a.removeStartupItem()
	} else {
		a.addStartupItem()
	}
	go a.sendState(a.currentState)
}
