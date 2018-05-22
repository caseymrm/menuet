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
	"sync"
	"unsafe"
)

// UserDefaults represents stored defaults
type UserDefaults struct {
	strings map[string]string
	ints    map[string]int
}

var defaultsInstance *UserDefaults
var defaultsOnce sync.Once

// Defaults returns the userDefaults singleton
func Defaults() *UserDefaults {
	defaultsOnce.Do(func() {
		defaultsInstance = &UserDefaults{
			strings: make(map[string]string),
			ints:    make(map[string]int),
		}
	})
	return defaultsInstance
}

// SetString sets a string default
func (u *UserDefaults) SetString(key, value string) {
	ckey := C.CString(string(key))
	cvalue := C.CString(string(value))
	C.setString(ckey, cvalue)
	C.free(unsafe.Pointer(ckey))
	C.free(unsafe.Pointer(cvalue))
	u.strings[key] = value
}

// String gets a string default, "" if not set
func (u *UserDefaults) String(key string) string {
	val, ok := u.strings[key]
	if ok {
		return val
	}
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
	u.ints[key] = value
}

// Integer gets a integer default, 0 if not set
func (u *UserDefaults) Integer(key string) int {
	val, ok := u.ints[key]
	if ok {
		return val
	}
	ckey := C.CString(string(key))
	value := C.getInteger(ckey)
	C.free(unsafe.Pointer(ckey))
	return int(value)
}
