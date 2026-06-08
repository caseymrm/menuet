#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

#import "NSImage+Resize.h"
#import "menuet.h"

void itemClicked(const char *);
void notificationRespond(const char *, const char *);
const char *children(const char *);
const char *searchResults(const char *, const char *);
void menuClosed(const char *);
bool hideStartup();
char *startAtLoginLabel();
char *quitLabel();
bool runningAtStartup();
void toggleStartup();
void shutdownWait();
void initNotifications(void);
bool hasTopLevelClicked();
void topLevelClicked();

// Tag applied to dynamically-inserted search result NSMenuItems. populate:
// clears items with this tag before rebuilding so a menu refresh doesn't
// confuse search results with user-supplied items.
#define MENUET_SEARCH_RESULT_TAG 31337

@class MenuetMenu;

NSStatusItem *_statusItem;
MenuetMenu *_rootMenu;

@interface MenuetSearchView : NSView <NSSearchFieldDelegate>
@property(nonatomic, strong) NSSearchField *field;
@property(nonatomic, copy) NSString *searchUnique;
@property(nonatomic, copy) NSString *savedQuery;
@property(nonatomic, assign) NSMenu *trackingMenu;
@property(nonatomic, assign) CGPoint cursorBeforeWarp;
@property(nonatomic, assign) BOOL cursorWasWarped;
- (instancetype)initWithPlaceholder:(NSString *)placeholder
                        searchUnique:(NSString *)searchUnique;
- (void)updatePlaceholder:(NSString *)placeholder searchUnique:(NSString *)unique;
- (void)applyQuery:(NSString *)query;
@end

@interface MenuetMenu : NSMenu <NSMenuDelegate>

@property(nonatomic, copy) NSString *unique;
@property(nonatomic, assign) BOOL root;
@property(nonatomic, assign) BOOL open;

@end

@implementation MenuetMenu
- (id)init {
	self = [super init];
	if (self) {
		self.delegate = self;
		self.autoenablesItems = false;
	}
	return self;
}

- (void)refreshVisibleMenus {
	if (!self.open) {
		return;
	}
	[self menuWillOpen:self];
	for (NSMenuItem *item in self.itemArray) {
		MenuetMenu *menu = (MenuetMenu *)item.submenu;
		if (menu != NULL) {
			[menu refreshVisibleMenus];
		}
	}
}

- (void)populate:(NSArray *)items {
	// Search-result items are inserted dynamically by MenuetSearchView and
	// aren't part of the user's children() output. Pull them out first so
	// the index-keyed reuse below doesn't confuse them with user items.
	NSMutableArray<NSMenuItem *> *staleResults = [NSMutableArray new];
	for (NSMenuItem *existing in self.itemArray) {
		if (existing.tag == MENUET_SEARCH_RESULT_TAG) {
			[staleResults addObject:existing];
		}
	}
	for (NSMenuItem *r in staleResults) {
		[self removeItem:r];
	}

	for (int i = 0; i < items.count; i++) {
		NSMenuItem *item = nil;
		if (i < self.numberOfItems) {
			item = [self itemAtIndex:i];
		}
		NSDictionary *dict = [items objectAtIndex:i];
		NSString *type = dict[@"Type"];
		if ([type isEqualTo:@"separator"]) {
			if (!item || !item.isSeparatorItem) {
				[self insertItem:[NSMenuItem separatorItem] atIndex:i];
			}
			continue;
		}
		if ([type isEqualTo:@"search"]) {
			NSString *placeholder = dict[@"Text"] ?: @"";
			NSString *searchUnique = dict[@"Unique"] ?: @"";
			BOOL reuse = item && [item.view isKindOfClass:NSClassFromString(@"MenuetSearchView")];
			if (!reuse) {
				if (item) [self removeItemAtIndex:i];
				item = [self insertItemWithTitle:@"" action:nil keyEquivalent:@"" atIndex:i];
				MenuetSearchView *searchView = [[MenuetSearchView alloc]
				                initWithPlaceholder:placeholder
				                       searchUnique:searchUnique];
				item.view = searchView;
			} else {
				MenuetSearchView *searchView = (MenuetSearchView *)item.view;
				[searchView updatePlaceholder:placeholder searchUnique:searchUnique];
			}
			// Defer the actual applyQuery to AFTER the while-loop trim
			// below — otherwise the trim wipes any results we'd insert
			// here (items.count == 1 for "[Search]"; the loop would trim
			// every search result back out).
			continue;
		}
		NSString *unique = dict[@"Unique"];
		NSString *text = dict[@"Text"];
		NSString *imageName = dict[@"Image"];
		NSNumber *fontSize = dict[@"FontSize"];
		NSNumber *fontWeight = dict[@"FontWeight"];
		BOOL state = [dict[@"State"] boolValue];
		BOOL hasChildren = [dict[@"HasChildren"] boolValue];
		BOOL clickable = [dict[@"Clickable"] boolValue];
		if (!item || item.isSeparatorItem) {
			item =
				[self insertItemWithTitle:@"" action:nil keyEquivalent:@"" atIndex:i];
		}
		NSMutableDictionary *attributes = [NSMutableDictionary new];
		float size = fontSize.floatValue;
		if (fontSize == 0) {
			size = 14;
		}
		attributes[NSFontAttributeName] =
			[NSFont monospacedDigitSystemFontOfSize:size
			 weight:fontWeight.floatValue];
		item.attributedTitle =
			[[NSMutableAttributedString alloc] initWithString:text
			 attributes:attributes];
		item.target = self;
		if (clickable) {
			item.action = @selector(press:);
			item.representedObject = unique;
		} else {
			item.action = nil;
			item.representedObject = nil;
		}
		if (state) {
			item.state = NSControlStateValueOn;
		} else {
			item.state = NSControlStateValueOff;
		}
		if (hasChildren) {
			if (!item.submenu) {
				item.submenu = [MenuetMenu new];
			}
			MenuetMenu *menu = (MenuetMenu *)item.submenu;
			menu.unique = unique;
		} else if (item.submenu) {
			item.submenu = nil;
		}
		item.enabled = clickable || hasChildren;
		item.image = [NSImage imageFromName:imageName withHeight:16];
	}
	while (self.numberOfItems > items.count) {
		[self removeItemAtIndex:self.numberOfItems - 1];
	}

	// Search results live OUTSIDE the user's items array — they're driven
	// dynamically by the field's query. Apply each search view's saved (or
	// empty) query here, after the trim, so the inserted results survive.
	for (NSMenuItem *it in [self.itemArray copy]) {
		if ([it.view isKindOfClass:NSClassFromString(@"MenuetSearchView")]) {
			MenuetSearchView *sv = (MenuetSearchView *)it.view;
			sv.field.stringValue = sv.savedQuery ?: @"";
			[sv applyQuery:sv.field.stringValue];
		}
	}
}

// The documentation says not to make changes here, but it seems to work.
// submenuAction does not appear to be called, and menuNeedsUpdate is only
// called once per tracking session.
- (void)menuWillOpen:(MenuetMenu *)menu {
	if (self.root) {
		// For the root menu, we generate a new unique every time it's opened. Go
		// handles all other unique generation.
		self.unique = [[[[NSProcessInfo processInfo] globallyUniqueString]
		                substringFromIndex:51] stringByAppendingString:@":root"];
	}
	const char *str = children(self.unique.UTF8String);
	NSArray *items = @[];
	if (str != NULL) {
		items = [NSJSONSerialization
		         JSONObjectWithData:[[NSString stringWithUTF8String:str]
		                             dataUsingEncoding:NSUTF8StringEncoding]
		         options:0
		         error:nil];
		free((char *)str);
	}
	if (self.root) {
		items = [items arrayByAddingObjectsFromArray:@[
				 @{@"Type" : @"separator",
				   @"Clickable" : @YES},
		]];
		if (!hideStartup()) {
			char *startLabel = startAtLoginLabel();
			items = [items arrayByAddingObjectsFromArray:@[
					@{@"Text" : [NSString stringWithUTF8String:startLabel],
					@"Clickable" : @YES},
			]];
			free(startLabel);
		}
		char *qLabel = quitLabel();
		items = [items arrayByAddingObjectsFromArray:@[
				 @{@"Text" : [NSString stringWithUTF8String:qLabel],
				   @"Clickable" : @YES},
		]];
		free(qLabel);
	}
	[self populate:items];
	if (self.root) {
		NSMenuItem *item = nil;
		if (!hideStartup()) {
			item = [self itemAtIndex:items.count - 2];
			item.action = @selector(toggleStartup:);
			if (runningAtStartup()) {
				item.state = NSControlStateValueOn;
			} else {
				item.state = NSControlStateValueOff;
			}
		}
		item = [self itemAtIndex:items.count - 1];
		item.action = @selector(prepareShutdown:);
	}
	self.open = YES;
}

- (void)menuDidClose:(MenuetMenu *)menu {
	self.open = NO;
	menuClosed(self.unique.UTF8String);
}

- (void)press:(id)sender {
	NSString *callback = [sender representedObject];
	itemClicked(callback.UTF8String);
}

- (void)toggleStartup:(id)sender {
	toggleStartup();
}

- (void)prepareShutdown:(id)sender {
	shutdownWait();
	[NSApp terminate: nil];
}

@end

// MenuetSearchView hosts an NSSearchField inside an NSMenuItem.
//
// AppKit's menu subsystem runs its own event-tracking loop that swallows
// keystrokes before NSResponder routing has a chance to deliver them to
// any embedded NSTextField. The accepted workaround (since 2017) is to
// install a Carbon event handler on the application's event dispatcher:
// keystrokes still flow through Carbon, even inside menu tracking, so
// we can intercept them and write into the NSSearchField ourselves.
//
// Carbon UI APIs are deprecated, but the Carbon Event Manager subset we
// use here (InstallEventHandler, GetEventDispatcherTarget, EventRef ->
// NSEvent via +eventWithEventRef:) is what AppKit itself relies on for
// NSMenu and remains unmarked-deprecated.
//
// Approach and key-passthrough list adapted from
// Interface declared up at top of file so MenuetMenu's populate: can
// touch it during menuWillOpen.

@implementation MenuetSearchView

- (instancetype)initWithPlaceholder:(NSString *)placeholder
                        searchUnique:(NSString *)searchUnique {
	self = [super initWithFrame:NSMakeRect(0, 0, 240, 28)];
	if (self) {
		_searchUnique = [searchUnique copy];
		_savedQuery = @"";
		_field = [[NSSearchField alloc] initWithFrame:NSMakeRect(6, 2, 228, 24)];
		_field.placeholderString = placeholder;
		_field.delegate = self;
		_field.autoresizingMask = NSViewWidthSizable;
		[self addSubview:_field];
	}
	return self;
}

- (void)updatePlaceholder:(NSString *)placeholder searchUnique:(NSString *)unique {
	if (![self.field.placeholderString isEqualToString:placeholder]) {
		self.field.placeholderString = placeholder;
	}
	// searchUnique changes every menu open (fresh uuid). Don't clobber the
	// field — savedQuery is what should persist.
	self.searchUnique = [unique copy];
}

- (void)dealloc {
	if (_trackingMenu) {
		[[NSNotificationCenter defaultCenter] removeObserver:self];
		_trackingMenu = nil;
	}
	[_field release];
	[_searchUnique release];
	[_savedQuery release];
	[super dealloc];
}

- (void)viewDidMoveToWindow {
	[super viewDidMoveToWindow];
	if (self.window == nil) {
		return;
	}

	// Subscribe once to end-tracking so we can persist the field's text
	// across menu opens. The notification is posted by the *root* menu
	// (not individual submenus), so we don't filter by object.
	if (!self.trackingMenu) {
		self.trackingMenu = self.enclosingMenuItem.menu;
		[[NSNotificationCenter defaultCenter]
		    addObserver:self
		       selector:@selector(menuDidEndTracking:)
		           name:NSMenuDidEndTrackingNotification
		         object:nil];
	}

	[self.window makeFirstResponder:self.field];

	// NSPopupMenuWindow refuses regular makeKeyWindow during menu tracking,
	// so the text field's input context never engages — no cursor blink,
	// no edit state. Calling the window's private -setKeyOverride:YES
	// flips its key-state override and lets the field's editor become
	// active. This is the first of two private hooks the Search type
	// relies on; documented as an App Store-incompatible caveat.
	SEL setKeyOverride = NSSelectorFromString(@"setKeyOverride:");
	if ([self.window respondsToSelector:setKeyOverride]) {
		NSMethodSignature *sig = [self.window methodSignatureForSelector:setKeyOverride];
		NSInvocation *inv = [NSInvocation invocationWithMethodSignature:sig];
		inv.selector = setKeyOverride;
		BOOL yes = YES;
		[inv setArgument:&yes atIndex:2];
		[inv invokeWithTarget:self.window];
	}

	// Begin an editing session on the field via -selectWithFrame:...
	// (the non-modal partner of -editWithFrame:...:event:, which spins a
	// modal event loop and freezes the menu). This gives us a visible text
	// cursor immediately, plus a selected range so any remembered query
	// is replaced when the user starts typing.
	NSText *editor = [self.window fieldEditor:YES forObject:self.field];
	NSUInteger len = self.field.stringValue.length;
	[self.field.cell selectWithFrame:self.field.bounds
	                         inView:self.field
	                         editor:editor
	                       delegate:self.field
	                          start:0
	                         length:len];

	// Engage NSMenu's "this menu item is actively selected" state for the
	// search field by synthesizing a hardware-level click on it. Arrow-key
	// navigation through the result items depends on this state; without
	// the click, NSMenu's tracking loop reads arrows as menu-nav.
	//
	// Constraints we have to thread:
	//   - NSMenu only accepts this engagement during the initial tracking-
	//     setup window, so we click on menu open, not on first keystroke.
	//   - It validates the click against the cursor's LIVE position, so
	//     the cursor must be inside the field's rect. We pin x to the
	//     field's left edge (closest to where the parent menu's hover sat)
	//     to minimize the visible motion.
	//   - The 150ms delay gives the submenu's window time to finish
	//     positioning; if we race the layout, the field's screen rect
	//     comes back garbage and we'd fling the cursor off-screen.
	//   - The cursor stays at the field for the menu's entire lifetime —
	//     NSMenu would revoke the active state the moment it leaves. We
	//     restore the cursor's original position in -menuDidEndTracking:.
	//   - We hide the cursor via -setHiddenUntilMouseMoves: AFTER the warp.
	//     Called before, the warp itself counts as movement and immediately
	//     reinstates the cursor; after, it stays hidden until the user
	//     physically moves the mouse.
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 150 * NSEC_PER_MSEC),
	               dispatch_get_main_queue(), ^{
		if (!self.window) return;
		NSRect fieldRectInWindow = [self.field convertRect:self.field.bounds
		                                            toView:nil];
		NSRect fieldRectOnScreen = [self.window convertRectToScreen:fieldRectInWindow];
		CGFloat maxY = 0;
		for (NSScreen *s in NSScreen.screens) {
			CGFloat top = NSMaxY(s.frame);
			if (top > maxY) maxY = top;
		}
		const CGFloat inset = 4;
		CGFloat minX = fieldRectOnScreen.origin.x + inset;
		CGFloat cgMinY = maxY - NSMaxY(fieldRectOnScreen) + inset;
		CGFloat cgMaxY = maxY - fieldRectOnScreen.origin.y - inset;

		CGEventRef probe = CGEventCreate(NULL);
		CGPoint orig = CGEventGetLocation(probe);
		CFRelease(probe);

		CGPoint target = orig;
		target.x = minX;
		if (target.y < cgMinY) target.y = cgMinY;
		if (target.y > cgMaxY) target.y = cgMaxY;

		// If the computed target is off-screen, the submenu's window had
		// not finished positioning. Bail rather than fling the cursor.
		// Arrow nav won't work for this particular open.
		if (target.y < 0 || target.y > maxY || target.x < 0) {
			return;
		}

		BOOL needsWarp = (target.x != orig.x) || (target.y != orig.y);
		self.cursorBeforeWarp = orig;
		self.cursorWasWarped = needsWarp;

		if (needsWarp) {
			CGWarpMouseCursorPosition(target);
			[NSCursor setHiddenUntilMouseMoves:YES];
		}
		CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown,
		                                          target, kCGMouseButtonLeft);
		CGEventPost(kCGSessionEventTap, down);
		CFRelease(down);
		CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp,
		                                        target, kCGMouseButtonLeft);
		CGEventPost(kCGSessionEventTap, up);
		CFRelease(up);
	});
}

- (void)menuDidEndTracking:(NSNotification *)note {
	self.savedQuery = self.field.stringValue;
	if (self.cursorWasWarped) {
		CGWarpMouseCursorPosition(self.cursorBeforeWarp);
		self.cursorWasWarped = NO;
	}
}

// NSTextFieldDelegate: live filtering as the user types.
- (void)controlTextDidChange:(NSNotification *)note {
	[self applyQuery:self.field.stringValue];
}

// NSTextFieldDelegate: intercept specific keys so Enter activates the first
// result, Esc closes the menu, and Down hands focus back to NSMenu so its
// arrow-key navigation can walk the result items.
- (BOOL)control:(NSControl *)control
       textView:(NSTextView *)textView
doCommandBySelector:(SEL)cmd {
	NSMenu *menu = self.enclosingMenuItem.menu;
	if (cmd == @selector(insertNewline:)) {
		NSMenuItem *first = [self firstActionableResult];
		if (first) {
			[NSApp sendAction:first.action to:first.target from:first];
			[menu cancelTracking];
		}
		return YES;
	}
	if (cmd == @selector(cancelOperation:)) {
		[menu cancelTracking];
		return YES;
	}
	if (cmd == @selector(moveDown:)) {
		// Relinquish first responder so NSMenu's own tracking handles arrows.
		[self.window makeFirstResponder:self.window];
		return YES;
	}
	return NO;
}

- (void)applyQuery:(NSString *)query {
	const char *json = searchResults(self.searchUnique.UTF8String, query.UTF8String);
	NSArray *items = @[];
	if (json != NULL) {
		NSData *data = [[NSString stringWithUTF8String:json]
		                dataUsingEncoding:NSUTF8StringEncoding];
		items = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil] ?: @[];
		free((char *)json);
	}

	NSMenuItem *enclosing = self.enclosingMenuItem;
	NSMenu *menu = enclosing.menu;
	if (!menu) {
		return;
	}

	// The menu's tag is the source of truth for what's a search result.
	// AppKit cycles view-window state multiple times per open, and
	// populate: can also pull tagged items out from under us — re-deriving
	// from menu.itemArray each time absorbs all of that without modeling
	// AppKit's internals.
	NSMutableArray<NSMenuItem *> *stale = [NSMutableArray new];
	for (NSMenuItem *existing in menu.itemArray) {
		if (existing.tag == MENUET_SEARCH_RESULT_TAG) {
			[stale addObject:existing];
		}
	}
	for (NSMenuItem *r in stale) {
		[menu removeItem:r];
	}

	// Insert fresh items below the search field.
	NSInteger insertIndex = [menu indexOfItem:enclosing] + 1;
	for (NSDictionary *dict in items) {
		NSString *type = dict[@"Type"];
		NSMenuItem *newItem;
		if ([type isEqual:@"separator"]) {
			newItem = [NSMenuItem separatorItem];
		} else {
			NSString *text = dict[@"Text"] ?: @"";
			BOOL clickable = [dict[@"Clickable"] boolValue];
			newItem = [[NSMenuItem alloc] initWithTitle:text action:nil keyEquivalent:@""];
			if (clickable) {
				newItem.target = menu;
				newItem.action = @selector(press:);
				newItem.representedObject = dict[@"Unique"];
			}
		}
		newItem.tag = MENUET_SEARCH_RESULT_TAG;
		[menu insertItem:newItem atIndex:insertIndex];
		insertIndex++;
	}

	// Force NSMenu to recompute its layout. Without this, items inserted
	// during tracking can render but the menu doesn't grow — the new rows
	// end up outside the visible content rect.
	[menu update];
}

// Returns the first tagged search-result item in this view's menu that has
// an action — used by the Enter key handler.
- (NSMenuItem *)firstActionableResult {
	NSMenu *menu = self.enclosingMenuItem.menu;
	for (NSMenuItem *it in menu.itemArray) {
		if (it.tag == MENUET_SEARCH_RESULT_TAG && it.action != NULL) {
			return it;
		}
	}
	return nil;
}

@end

@interface MenuetAppDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate, UNUserNotificationCenterDelegate>

@end

void setState(const char *jsonString) {
	NSDictionary *state = [NSJSONSerialization
	                       JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
	                                           dataUsingEncoding:NSUTF8StringEncoding]
	                       options:0
	                       error:nil];
	dispatch_async(dispatch_get_main_queue(), ^{
		_statusItem.button.attributedTitle = [[NSAttributedString alloc]
		                                      initWithString:state[@"Title"]
		                                      attributes:@{
		                                              NSFontAttributeName :
		                                              [NSFont monospacedDigitSystemFontOfSize:14
		                                               weight:NSFontWeightRegular]
		}];
		NSString *imageName = state[@"Image"];
		NSImage *image = [NSImage imageFromName:imageName withHeight:22];
		_statusItem.button.image = image;
		_statusItem.button.image.template = true;
		_statusItem.button.imagePosition = NSImageLeft;
	});
}

void menuChanged() {
        dispatch_async(dispatch_get_main_queue(), ^{
		[_rootMenu refreshVisibleMenus];
	});
}

void createAndRunApplication() {
        [NSAutoreleasePool new];
        NSApplication *a = NSApplication.sharedApplication;
        MenuetAppDelegate *d = [MenuetAppDelegate new];
        [a setDelegate:d];
        initNotifications();
        [UNUserNotificationCenter currentNotificationCenter].delegate = d;
        [a setActivationPolicy:NSApplicationActivationPolicyAccessory];
        _statusItem = [[NSStatusBar systemStatusBar]
                       statusItemWithLength:NSVariableStatusItemLength];
        _rootMenu = [MenuetMenu new];
        _rootMenu.root = true;
        // We intercept all button clicks instead of assigning _statusItem.menu
        // so that the optional Application.Clicked handler (Go side) can fire
        // on left clicks while the menu still opens on right click. When no
        // handler is set, every click falls through to the menu popup.
        _statusItem.button.target = d;
        _statusItem.button.action = @selector(statusItemClicked:);
        [_statusItem.button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
        [a run];
}

@implementation MenuetAppDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:
        (NSApplication *)sender {
        return NSTerminateNow;
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       didReceiveNotificationResponse:(UNNotificationResponse *)response
       withCompletionHandler:(void (^)(void))completionHandler {
        NSString *identifier = response.notification.request.identifier;
        if ([response isKindOfClass:[UNTextInputNotificationResponse class]]) {
                NSString *userText = ((UNTextInputNotificationResponse *)response).userText;
                notificationRespond(identifier.UTF8String, userText.UTF8String);
        } else {
                notificationRespond(identifier.UTF8String, @"".UTF8String);
        }
        completionHandler();
}

- (void)statusItemClicked:(id)sender {
        NSEvent *event = [NSApp currentEvent];
        BOOL openMenu = !hasTopLevelClicked() ||
                        event.type == NSEventTypeRightMouseUp ||
                        (event.modifierFlags & NSEventModifierFlagControl) != 0;
        if (openMenu) {
                NSStatusBarButton *button = _statusItem.button;
                button.highlighted = YES;
                [_rootMenu popUpMenuPositioningItem:nil
                                         atLocation:NSMakePoint(0, button.bounds.size.height)
                                             inView:button];
                button.highlighted = NO;
                return;
        }
        topLevelClicked();
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
       withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
        if (@available(macOS 11.0, *)) {
                completionHandler(UNNotificationPresentationOptionBanner |
                                  UNNotificationPresentationOptionSound);
        } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
                completionHandler(UNNotificationPresentationOptionAlert |
                                  UNNotificationPresentationOptionSound);
#pragma clang diagnostic pop
        }
}

@end
