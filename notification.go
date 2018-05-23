package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef __NOTIFICATION_H_H__
#import "notification.h"
#endif

void showNotification(const char *jsonString);

*/
import "C"
import (
	"encoding/json"
	"log"
	"unsafe"
)

// Notification represents an NSUserNotification
type Notification struct {
	Title    string
	Subtitle string
	Message  string
}

// Notification shows a notification to the user. Note that you have to be part of a proper application bundle for them to show up.
func (a *Application) Notification(notification Notification) {
	b, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Marshal: %v", err)
		return
	}
	cstr := C.CString(string(b))
	C.showNotification(cstr)
	C.free(unsafe.Pointer(cstr))
}
