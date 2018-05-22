package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef __MENUET_H_H__
#import "menuet.h"
#endif

void setState(const char *jsonString);
void createAndRunApplication();

*/
import "C"
import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"
	"unsafe"
)

// ItemType represents what type of menu item this is
type ItemType string

const (
	// Regular is a normal item with text and optional callback
	Regular ItemType = ""
	// Separator is a horizontal line
	Separator = "separator"
	// TODO: StartAtLogin, Quit
)

// MenuItem represents one item in the dropdown
type MenuItem struct {
	Type ItemType
	// These fields only used for Regular item type:
	Text       string
	FontSize   int
	FontWeight int
	Callback   string
	State      bool // checkmark if true
	Children   []MenuItem
}

// MenuState represents the title and drop down,
type MenuState struct {
	Title string
	// This is the name of an image in the Resources directory
	Image string
	Items []MenuItem
}

// Application represents the OSX application
type Application struct {
	Name  string
	Label string

	// Clicked receives callbacks of menu items selected
	// It discards messages if the channel is not ready for them
	Clicked    chan<- string
	MenuOpened func() []MenuItem

	alertChannel       chan int
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
	C.createAndRunApplication()
}

// SetMenuState changes what is shown in the dropdown
func (a *Application) SetMenuState(state *MenuState) {
	if reflect.DeepEqual(a.currentState, state) {
		return
	}
	go a.sendState(state)
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
		log.Printf("Marshal: %v", err)
		return
	}
	cstr := C.CString(string(b))
	C.setState(cstr)
	C.free(unsafe.Pointer(cstr))
}

// Alert shows an alert, and returns the index of the button pressed, or -1 if none
func (a *Application) Alert(messageText, informativeText string, buttons ...string) int {
	if a.alertChannel != nil {
		log.Printf("Alert message already showing")
		return -1
	}
	b, err := json.Marshal(struct {
		MessageText     string
		InformativeText string
		Buttons         []string
	}{
		messageText,
		informativeText,
		buttons,
	})
	if err != nil {
		log.Printf("Marshal: %v", err)
		return -1
	}
	cstr := C.CString(string(b))
	C.showAlert(cstr)
	C.free(unsafe.Pointer(cstr))
	a.alertChannel = make(chan int)
	response := <-a.alertChannel
	a.alertChannel = nil
	return response
}

func (a *Application) clicked(callback string) {
	if a.Clicked == nil {
		return
	}
	select {
	case a.Clicked <- callback:
	default:
		fmt.Printf("dropped %s click", callback)
	}
}

func (a *Application) menuOpened() []MenuItem {
	if a.MenuOpened == nil {
		return nil
	}
	return a.MenuOpened()
}

//export itemClicked
func itemClicked(callbackCString *C.char) {
	callback := C.GoString(callbackCString)
	App().clicked(callback)
}

//export alertClicked
func alertClicked(button int) {
	app := App()
	if app.alertChannel == nil {
		log.Printf("Alert message double clicked?")
		return
	}
	app.alertChannel <- button
}

//export menuOpened
func menuOpened() *C.char {
	items := App().menuOpened()
	if items == nil {
		return nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		log.Printf("Marshal: %v", err)
		return nil
	}
	App().currentState.Items = items
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
