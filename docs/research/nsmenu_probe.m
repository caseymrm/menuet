// Probe AppKit's NSMenu / NSMenuItem / NSPopupMenuWindow for any private
// selectors that look related to search, filtering, highlight, or focus.
// Build:  clang -framework Cocoa -framework AppKit nsmenu_probe.m -o nsmenu_probe
// Run:    ./nsmenu_probe

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

static void dumpClass(const char *name, NSString *grep) {
    Class cls = objc_getClass(name);
    if (!cls) {
        printf("=== %s NOT FOUND ===\n", name);
        return;
    }
    printf("=== %s instance methods matching /%s/ ===\n", name, grep.UTF8String);

    // Walk the class chain to catch methods inherited from private internals.
    Class current = cls;
    while (current && current != [NSObject class]) {
        unsigned int count = 0;
        Method *methods = class_copyMethodList(current, &count);
        for (unsigned int i = 0; i < count; i++) {
            const char *sel = sel_getName(method_getName(methods[i]));
            NSString *s = [NSString stringWithUTF8String:sel];
            if ([s rangeOfString:grep options:NSCaseInsensitiveSearch].location != NSNotFound) {
                printf("  %s :: %s\n", class_getName(current), sel);
            }
        }
        free(methods);
        current = class_getSuperclass(current);
    }

    printf("=== %s class methods matching /%s/ ===\n", name, grep.UTF8String);
    current = object_getClass(cls);
    while (current && current != object_getClass([NSObject class])) {
        unsigned int count = 0;
        Method *methods = class_copyMethodList(current, &count);
        for (unsigned int i = 0; i < count; i++) {
            const char *sel = sel_getName(method_getName(methods[i]));
            NSString *s = [NSString stringWithUTF8String:sel];
            if ([s rangeOfString:grep options:NSCaseInsensitiveSearch].location != NSNotFound) {
                printf("  +[%s %s]\n", class_getName(current), sel);
            }
        }
        free(methods);
        current = class_getSuperclass(current);
    }
}

static void listAllAppKitClasses(NSString *grep) {
    printf("=== AppKit classes matching /%s/ ===\n", grep.UTF8String);
    unsigned int total = 0;
    int *classes = (int *)objc_copyClassNamesForImage(
        "/System/Library/Frameworks/AppKit.framework/Versions/C/AppKit", &total);
    if (!classes) {
        // dyld_shared_cache path on newer macOS
        printf("(no image-specific class list; falling back to objc_getClassList)\n");
        int count = objc_getClassList(NULL, 0);
        Class *all = (Class *)malloc(sizeof(Class) * count);
        objc_getClassList(all, count);
        for (int i = 0; i < count; i++) {
            const char *n = class_getName(all[i]);
            NSString *s = [NSString stringWithUTF8String:n];
            if ([s rangeOfString:grep options:NSCaseInsensitiveSearch].location != NSNotFound) {
                printf("  %s\n", n);
            }
        }
        free(all);
        return;
    }
    free(classes);
}

int main(int argc, char *argv[]) {
    @autoreleasepool {
        // Names of interest
        const char *targets[] = {
            "NSMenu", "NSMenuItem", "NSPopupMenuWindow", "NSCarbonMenuImpl",
            "NSMenuTrackingData", "NSContextMenuImpl", "NSHelpManager",
            "NSCarbonMenuWindow", "NSMenuView",
        };
        NSArray<NSString *> *greps = @[
            @"search", @"filter", @"highlight",
            @"track", @"key", @"first", @"focus",
        ];

        for (size_t t = 0; t < sizeof(targets)/sizeof(targets[0]); t++) {
            for (NSString *g in greps) {
                dumpClass(targets[t], g);
            }
        }

        // Also dump all classes whose name contains any interesting keyword.
        listAllAppKitClasses(@"Help");
        listAllAppKitClasses(@"Search");
    }
    return 0;
}
