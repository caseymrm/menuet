package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications

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

// Notification represents a macOS user notification.
type Notification struct {
	// The basic text of the notification
	Title    string
	Subtitle string
	Message  string

	// These add an optional action button, configure dismiss behavior, and add an in-line reply.
	// Note: on macOS 11+, CloseButton still causes the dismiss action to trigger the
	// NotificationResponder callback, but custom button text is not supported by the
	// UserNotifications framework — the system default text is used instead.
	ActionButton        string
	CloseButton         string
	ResponsePlaceholder string

	// Duplicate identifiers do not re-display, but instead update the notification center
	Identifier string

	// If true, the notification is shown, but then deleted from the notification center
	RemoveFromNotificationCenter bool
}

func runningInAppBundle() bool {
	_, bundlePath := appPath()
	return bundlePath != ""
}

// Notification shows a notification to the user. Note that you have to be part of a proper application bundle for them to show up.
func (a *Application) Notification(notification Notification) {
	if !runningInAppBundle() {
		log.Printf("Warning: notifications won't show up unless running inside an application bundle")
	}
	b, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Marshal: %v", err)
		return
	}
	cstr := C.CString(string(b))
	C.showNotification(cstr)
	C.free(unsafe.Pointer(cstr))
}
