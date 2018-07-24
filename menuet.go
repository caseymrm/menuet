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
)

// Application represents the OSX application
type Application struct {
	Name  string
	Label string

	// Clicked is called with the key string of a menu item that is selected
	Clicked func(string)
	// MenuOpened is called to refresh menu items when clicked, empty string for the top level
	MenuOpened func(string) []MenuItem

	// If Version and Repo are set, checks for updates every day
	AutoUpdate struct {
		Version string
		Repo    string // For example "caseymrm/menuet"
	}

	alertChannel       chan AlertClicked
	currentState       *MenuState
	nextState          *MenuState
	pendingStateChange bool
	debounceMutex      sync.Mutex
}

var appInstance *Application
var appOnce sync.Once

// App returns the application singleton
func App() *Application {
	appOnce.Do(func() {
		appInstance = &Application{}
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

// ItemType represents what type of menu item this is
type ItemType string

const (
	// Regular is a normal item with text and optional callback
	Regular ItemType = ""
	// Separator is a horizontal line
	Separator = "separator"
	// TODO: StartAtLogin, Quit, Image, Spinner, etc
)

// MenuItem represents one item in the dropdown
type MenuItem struct {
	Type ItemType
	Key  string // Only required if Clickable or Children is true

	Text       string
	FontSize   int // Default: 14
	FontWeight FontWeight
	State      bool // shows checkmark when set
	Disabled   bool
	Children   bool
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

func (a *Application) clicked(key string) {
	if a.Clicked == nil {
		return
	}
	go a.Clicked(key)
}

func (a *Application) menuOpened(key string) []MenuItem {
	if a.MenuOpened == nil {
		return nil
	}
	return a.MenuOpened(key)
}

//export itemClicked
func itemClicked(keyCString *C.char) {
	key := C.GoString(keyCString)
	App().clicked(key)
}

//export menuOpened
func menuOpened(keyCString *C.char) *C.char {
	key := C.GoString(keyCString)
	items := App().menuOpened(key)
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
