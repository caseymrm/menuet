package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef __ALERT_H_H__
#import "alert.h"
#endif

void showAlert(const char *jsonString);

*/
import "C"
import (
	"encoding/json"
	"log"
	"unsafe"
)

// Alert represents an NSAlert
type Alert struct {
	MessageText     string
	InformativeText string
	Buttons         []string
}

// Alert shows an alert, and returns the index of the button pressed, or -1 if none
func (a *Application) Alert(alert Alert) int {
	if a.alertChannel != nil {
		log.Printf("Alert message already showing")
		return -1
	}
	b, err := json.Marshal(alert)
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

//export alertClicked
func alertClicked(button int) {
	app := App()
	if app.alertChannel == nil {
		log.Printf("Alert message double clicked?")
		return
	}
	app.alertChannel <- button
}
