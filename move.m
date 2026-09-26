#import <Cocoa/Cocoa.h>
#import <dlfcn.h>

#import "move.h"

// App Translocation (macOS 10.12+) runs a quarantined app that the user has
// not moved from a randomized read-only mirror. The Security framework
// resolves the mirror back to the real bundle, but only through SPI that is
// absent from the public SDK headers, so look it up at runtime as LetsMove
// does.
typedef Boolean (*SecTranslocateIsTranslocatedURLFn)(CFURLRef, bool *, CFErrorRef *);
typedef CFURLRef (*SecTranslocateCreateOriginalPathForURLFn)(CFURLRef, CFErrorRef *);

// menuetOriginalBundlePath returns the path of the bundle the user actually
// has on disk: path itself when it is not translocated, or the original
// location when it is. It returns NULL when path is translocated but the
// original cannot be resolved. The caller frees the result.
char *menuetOriginalBundlePath(const char *path, bool *translocated) {
	*translocated = false;
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		void *security = dlopen("/System/Library/Frameworks/Security.framework/Security", RTLD_LAZY);
		SecTranslocateIsTranslocatedURLFn isTranslocated = NULL;
		SecTranslocateCreateOriginalPathForURLFn createOriginal = NULL;
		if (security) {
			isTranslocated = (SecTranslocateIsTranslocatedURLFn)dlsym(security, "SecTranslocateIsTranslocatedURL");
			createOriginal = (SecTranslocateCreateOriginalPathForURLFn)dlsym(security, "SecTranslocateCreateOriginalPathForURL");
		}
		if (!isTranslocated || !createOriginal) {
			// Before 10.12 there is no translocation to resolve.
			return strdup(path);
		}
		bool result = false;
		if (!isTranslocated((CFURLRef)url, &result, NULL)) {
			return NULL;
		}
		if (!result) {
			return strdup(path);
		}
		*translocated = true;
		CFURLRef original = createOriginal((CFURLRef)url, NULL);
		if (!original) {
			return NULL;
		}
		char *originalPath = strdup(((NSURL *)original).path.fileSystemRepresentation);
		CFRelease(original);
		return originalPath;
	}
}

// menuetMoveAlert runs a modal alert synchronously on the calling (main)
// thread. It runs before [NSApp run], so it cannot use showAlert, which
// dispatches to the main queue and waits for Go. It returns the 0-based index
// of the button pressed.
int menuetMoveAlert(const char *message, const char *info,
                    const char *firstButton, const char *secondButton,
                    bool showSuppression, bool *suppressed) {
	@autoreleasepool {
		NSAlert *alert = [NSAlert new];
		alert.messageText = [NSString stringWithUTF8String:message];
		alert.informativeText = [NSString stringWithUTF8String:info];
		[alert addButtonWithTitle:[NSString stringWithUTF8String:firstButton]];
		if (secondButton) {
			[alert addButtonWithTitle:[NSString stringWithUTF8String:secondButton]];
		}
		if (showSuppression) {
			alert.showsSuppressionButton = YES;
			alert.suppressionButton.title = @"Do not ask again";
		}
		[NSApp activateIgnoringOtherApps:YES];
		NSInteger resp = [alert runModal];
		if (suppressed) {
			*suppressed = showSuppression && alert.suppressionButton.state == NSControlStateValueOn;
		}
		return (int)(resp - NSAlertFirstButtonReturn);
	}
}

// fileIdentity returns the file system's identity for path, or nil when it
// does not exist. Comparing identities instead of path strings matches case
// variants on a case-insensitive volume, and paths reached through symlinks.
static id fileIdentity(NSString *path) {
	id identity = nil;
	[[NSURL fileURLWithPath:path] getResourceValue:&identity forKey:NSURLFileResourceIdentifierKey error:nil];
	return identity;
}

// menuetOtherInstanceRunningAt reports whether another process with
// bundleID runs from the bundle at path. A translocated instance counts
// when its original is path.
bool menuetOtherInstanceRunningAt(const char *bundleID, const char *path) {
	@autoreleasepool {
		id target = fileIdentity([NSString stringWithUTF8String:path]);
		if (!target) {
			return false;
		}
		pid_t me = getpid();
		for (NSRunningApplication *app in [NSRunningApplication runningApplicationsWithBundleIdentifier:[NSString stringWithUTF8String:bundleID]]) {
			if (app.processIdentifier == me || !app.bundleURL) {
				continue;
			}
			NSString *running = app.bundleURL.path;
			bool translocated = false;
			char *original = menuetOriginalBundlePath(running.fileSystemRepresentation, &translocated);
			if (original) {
				running = [NSString stringWithUTF8String:original];
				free(original);
			}
			if ([fileIdentity(running) isEqual:target]) {
				return true;
			}
		}
		return false;
	}
}

// menuetTrash moves path to the user's Trash. It returns NULL on success or
// an error description that the caller frees.
char *menuetTrash(const char *path) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
		NSError *err = nil;
		if ([[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&err]) {
			return NULL;
		}
		return strdup((err.localizedDescription ?: @"unknown error").UTF8String);
	}
}
