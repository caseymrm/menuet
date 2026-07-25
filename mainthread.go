package menuet

import "runtime"

// AppKit requires that NSApplication and NSStatusItem be created on the
// process's main thread. RunApplication calls into Cocoa directly
// (C.createAndRunApplication), so whichever OS thread the main goroutine
// occupies at that moment IS the thread AppKit sees.
//
// Go makes no promise there. The main goroutine starts on the main thread but
// the scheduler may migrate it to another M at any preemption point, so
// menuet's main-thread requirement was satisfied only by luck: apps that did
// little before RunApplication usually survived, and apps that did some cgo or
// channel work first sometimes died at startup with
//
//	NSInternalInconsistencyException: NSWindow should only be instantiated
//	on the main thread!
//
// ...from inside -[NSStatusBar _statusItemWithLength:]. Intermittent, so it
// read as a flaky app rather than a library invariant. It cost a downstream app
// five days of being silently dead after one innocuous line was added ahead of
// RunApplication.
//
// LockOSThread in an init function is the documented fix: per runtime's docs,
// "calling LockOSThread from an init function will cause the main function to
// be invoked on that thread" — pinning the main goroutine to the main OS
// thread for the life of the process. Every menuet app gets this by importing
// the package, which is right: the constraint belongs to the library that
// calls AppKit, not to each of its callers.
func init() {
	runtime.LockOSThread()
}
