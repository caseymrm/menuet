package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef TRAY_H
#import "tray.h"
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
	"unsafe"
)

// MenuItem represents one item in the dropdown
type MenuItem struct {
	Text     string // "---" is a separator
	Callback string
}

// MenuState represents the title and drop down,
type MenuState struct {
	Title string
	Items []MenuItem
}

// Application represents the OSX application
type Application struct {
	currentState *MenuState
}

var instance *Application
var appOnce sync.Once

// App returns the application singleton
func App() *Application {
	appOnce.Do(func() {
		instance = &Application{}
	})
	return instance
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
	b, err := json.Marshal(state)
	if err != nil {
		log.Printf("Marshal: %v", err)
		return
	}
	cstr := C.CString(string(b))
	C.setState(cstr)
	C.free(unsafe.Pointer(cstr))
	a.currentState = state
}

//export ItemClicked
func ItemClicked(callbackCString *C.char) {
	callback := C.GoString(callbackCString)
	fmt.Printf("Item Clicked %s\n", callback)
}
