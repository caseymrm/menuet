package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

#ifndef __USERDEFAULTS_H_H__
#import "userdefaults.h"
#endif

*/
import "C"
import (
	"unsafe"
)

// UserDefaults represents stored defaults
type UserDefaults struct {
	strings map[string]string
}

// SetString sets a string default
func (u *UserDefaults) SetString(key, value string) {
	ckey := C.CString(string(key))
	cvalue := C.CString(string(value))
	C.setString(ckey, cvalue)
	C.free(unsafe.Pointer(ckey))
	C.free(unsafe.Pointer(cvalue))
}

// String gets a string default, "" if not set
func (u *UserDefaults) String(key string) string {
	ckey := C.CString(string(key))
	cvalue := C.getString(ckey)
	value := C.GoString(cvalue)
	C.free(unsafe.Pointer(ckey))
	return value
}

// SetInteger sets a integer default
func (u *UserDefaults) SetInteger(key string, value int) {
	ckey := C.CString(string(key))
	C.setInteger(ckey, C.long(value))
	C.free(unsafe.Pointer(ckey))
}

// Integer gets a integer default, 0 if not set
func (u *UserDefaults) Integer(key string) int {
	ckey := C.CString(string(key))
	value := C.getInteger(ckey)
	C.free(unsafe.Pointer(ckey))
	return int(value)
}
