#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>
#import <Carbon/Carbon.h>
#import <ServiceManagement/ServiceManagement.h>

#import "NSImage+Resize.h"
#import "menuet.h"

void hotkeyFired(uint32_t id);

// SMAppService is macOS 13+. The wrapper functions return:
//   -1 = API unavailable on this macOS
//    0 = success / not registered
//    1 = registered / requires user approval
//    2 = registration error (e.g. unsigned bundle — caller should fall back)
//    3 = not found (status only)
int menuetSMAppServiceRegister(void) {
	if (@available(macOS 13.0, *)) {
		SMAppService *service = [SMAppService mainAppService];
		NSError *err = nil;
		BOOL ok = [service registerAndReturnError:&err];
		if (ok) return 0;
		if (err) {
			NSLog(@"menuet: SMAppService register failed: %@", err);
		}
		return 2;
	}
	return -1;
}

int menuetSMAppServiceUnregister(void) {
	if (@available(macOS 13.0, *)) {
		SMAppService *service = [SMAppService mainAppService];
		NSError *err = nil;
		BOOL ok = [service unregisterAndReturnError:&err];
		if (ok) return 0;
		if (err) {
			NSLog(@"menuet: SMAppService unregister failed: %@", err);
		}
		return 2;
	}
	return -1;
}

int menuetSMAppServiceStatus(void) {
	if (@available(macOS 13.0, *)) {
		SMAppService *service = [SMAppService mainAppService];
		switch (service.status) {
			case SMAppServiceStatusEnabled:           return 1;
			case SMAppServiceStatusRequiresApproval:  return 1;
			case SMAppServiceStatusNotRegistered:     return 0;
			case SMAppServiceStatusNotFound:          return 3;
		}
		return 0;
	}
	return -1;
}

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

// Global-hotkey Carbon plumbing. Each Shortcut registered from Go gets an
// EventHotKeyRef stored here keyed by the Go-assigned uint32 ID, so we can
// unregister on demand. The shared event handler dispatches incoming
// kEventHotKeyPressed events back into Go via hotkeyFired().
static NSMutableDictionary<NSNumber *, NSValue *> *MenuetHotkeyRefs;
static EventHandlerRef MenuetHotkeyHandler;

static OSStatus MenuetHotkeyEventCallback(EventHandlerCallRef handler,
                                          EventRef event,
                                          void *userData) {
	EventHotKeyID id;
	OSStatus status = GetEventParameter(event, kEventParamDirectObject,
	                                     typeEventHotKeyID, NULL,
	                                     sizeof(id), NULL, &id);
	if (status != noErr) return status;
	hotkeyFired(id.id);
	return noErr;
}

static void MenuetEnsureHotkeyHandler(void) {
	if (MenuetHotkeyHandler) return;
	MenuetHotkeyRefs = [NSMutableDictionary new];
	EventTypeSpec evt = { kEventClassKeyboard, kEventHotKeyPressed };
	InstallApplicationEventHandler(MenuetHotkeyEventCallback, 1, &evt,
	                                NULL, &MenuetHotkeyHandler);
}

void registerHotkey(uint32_t goID, uint32_t keyCode, uint32_t modifiers) {
	dispatch_async(dispatch_get_main_queue(), ^{
		MenuetEnsureHotkeyHandler();
		EventHotKeyID hkID = { .signature = 'menu', .id = goID };
		EventHotKeyRef ref = NULL;
		OSStatus status = RegisterEventHotKey(keyCode, modifiers, hkID,
		                                       GetApplicationEventTarget(),
		                                       0, &ref);
		if (status != noErr) {
			NSLog(@"menuet: RegisterEventHotKey failed (status=%d) for id=%u",
			       (int)status, goID);
			return;
		}
		MenuetHotkeyRefs[@(goID)] = [NSValue valueWithPointer:ref];
	});
}

void unregisterHotkey(uint32_t goID) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSValue *boxed = MenuetHotkeyRefs[@(goID)];
		if (!boxed) return;
		EventHotKeyRef ref = (EventHotKeyRef)boxed.pointerValue;
		UnregisterEventHotKey(ref);
		[MenuetHotkeyRefs removeObjectForKey:@(goID)];
	});
}

@class MenuetMenu;

NSStatusItem *_statusItem;
MenuetMenu *_rootMenu;

@interface MenuetSearchView : NSView <NSSearchFieldDelegate>
@property(nonatomic, strong) NSSearchField *field;
@property(nonatomic, copy) NSString *searchUnique;
@property(nonatomic, copy) NSString *savedQuery;
@property(nonatomic, assign) NSMenu *trackingMenu;
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

// Convert one of Apple's virtual key codes (the same KeyCode values our
// Go-side Key constants use) to the NSString form NSMenuItem.keyEquivalent
// expects. Returns @"" for codes we don't have a printable mapping for.
static NSString *MenuetKeyEquivalentStringForCode(int keyCode) {
	// Letter keys, layout-independent — match the Go-side constants.
	switch (keyCode) {
		case 0:  return @"a";
		case 11: return @"b";
		case 8:  return @"c";
		case 2:  return @"d";
		case 14: return @"e";
		case 3:  return @"f";
		case 5:  return @"g";
		case 4:  return @"h";
		case 34: return @"i";
		case 38: return @"j";
		case 40: return @"k";
		case 37: return @"l";
		case 46: return @"m";
		case 45: return @"n";
		case 31: return @"o";
		case 35: return @"p";
		case 12: return @"q";
		case 15: return @"r";
		case 1:  return @"s";
		case 17: return @"t";
		case 32: return @"u";
		case 9:  return @"v";
		case 13: return @"w";
		case 7:  return @"x";
		case 16: return @"y";
		case 6:  return @"z";

		case 29: return @"0";
		case 18: return @"1";
		case 19: return @"2";
		case 20: return @"3";
		case 21: return @"4";
		case 23: return @"5";
		case 22: return @"6";
		case 26: return @"7";
		case 28: return @"8";
		case 25: return @"9";

		case 49: return @" ";                      // space
		case 36: return [NSString stringWithFormat:@"%C", (unichar)NSCarriageReturnCharacter];
		case 48: return @"\t";
		case 53: return [NSString stringWithFormat:@"%C", (unichar)0x1B]; // escape

		case 122: return [NSString stringWithFormat:@"%C", (unichar)NSF1FunctionKey];
		case 120: return [NSString stringWithFormat:@"%C", (unichar)NSF2FunctionKey];
		case 99:  return [NSString stringWithFormat:@"%C", (unichar)NSF3FunctionKey];
		case 118: return [NSString stringWithFormat:@"%C", (unichar)NSF4FunctionKey];
		case 96:  return [NSString stringWithFormat:@"%C", (unichar)NSF5FunctionKey];
		case 97:  return [NSString stringWithFormat:@"%C", (unichar)NSF6FunctionKey];
		case 98:  return [NSString stringWithFormat:@"%C", (unichar)NSF7FunctionKey];
		case 100: return [NSString stringWithFormat:@"%C", (unichar)NSF8FunctionKey];
		case 101: return [NSString stringWithFormat:@"%C", (unichar)NSF9FunctionKey];
		case 109: return [NSString stringWithFormat:@"%C", (unichar)NSF10FunctionKey];
		case 103: return [NSString stringWithFormat:@"%C", (unichar)NSF11FunctionKey];
		case 111: return [NSString stringWithFormat:@"%C", (unichar)NSF12FunctionKey];

		case 123: return [NSString stringWithFormat:@"%C", (unichar)NSLeftArrowFunctionKey];
		case 124: return [NSString stringWithFormat:@"%C", (unichar)NSRightArrowFunctionKey];
		case 125: return [NSString stringWithFormat:@"%C", (unichar)NSDownArrowFunctionKey];
		case 126: return [NSString stringWithFormat:@"%C", (unichar)NSUpArrowFunctionKey];
	}
	return @"";
}

// Convert the Carbon-bit modifier mask we use in Go to the AppKit
// NSEventModifierFlag bits NSMenuItem.keyEquivalentModifierMask expects.
static NSEventModifierFlags MenuetModifierMaskFromCarbon(uint32_t carbon) {
	NSEventModifierFlags mask = 0;
	if (carbon & cmdKey)     mask |= NSEventModifierFlagCommand;
	if (carbon & shiftKey)   mask |= NSEventModifierFlagShift;
	if (carbon & optionKey)  mask |= NSEventModifierFlagOption;
	if (carbon & controlKey) mask |= NSEventModifierFlagControl;
	return mask;
}

// Build an NSAttributedString for a menu item's title. If runs is non-nil
// and non-empty, each run is appended with its own per-segment attributes
// (color, font size, weight, monospace). Otherwise the whole title is
// styled with the item-level attributes.
//
// A run's zero-value Color/FontSize/FontWeight means "inherit from the
// item-level value" so callers can change just one attribute per run
// without re-specifying the rest.
static NSColor *MenuetColorFromDict(NSDictionary *dict) {
	if (!dict) return nil;
	NSNumber *r = dict[@"R"];
	NSNumber *g = dict[@"G"];
	NSNumber *b = dict[@"B"];
	NSNumber *a = dict[@"A"];
	if (!r || !g || !b || !a) return nil;
	if (r.intValue == 0 && g.intValue == 0 && b.intValue == 0 && a.intValue == 0) {
		return nil;
	}
	return [NSColor colorWithRed:r.floatValue / 255.0
	                       green:g.floatValue / 255.0
	                        blue:b.floatValue / 255.0
	                       alpha:a.floatValue / 255.0];
}

static NSFont *MenuetFont(CGFloat size, CGFloat weight, BOOL mono) {
	if (size <= 0) size = 14;
	if (mono) {
		return [NSFont monospacedSystemFontOfSize:size weight:weight];
	}
	return [NSFont monospacedDigitSystemFontOfSize:size weight:weight];
}

static NSAttributedString *MenuetBuildAttributedTitle(NSString *text,
                                                      NSArray *runs,
                                                      CGFloat itemFontSize,
                                                      CGFloat itemFontWeight,
                                                      NSDictionary *itemColorDict,
                                                      BOOL itemMono) {
	NSColor *itemColor = MenuetColorFromDict(itemColorDict);
	if (![runs isKindOfClass:[NSArray class]] || runs.count == 0) {
		NSMutableDictionary *attrs = [NSMutableDictionary new];
		attrs[NSFontAttributeName] = MenuetFont(itemFontSize, itemFontWeight, itemMono);
		if (itemColor) attrs[NSForegroundColorAttributeName] = itemColor;
		return [[NSAttributedString alloc] initWithString:(text ?: @"") attributes:attrs];
	}
	NSMutableAttributedString *result = [NSMutableAttributedString new];
	for (NSDictionary *run in runs) {
		if (![run isKindOfClass:[NSDictionary class]]) continue;
		NSString *segText = run[@"Text"] ?: @"";
		if (segText.length == 0) continue;
		NSNumber *fsNum = run[@"FontSize"];
		NSNumber *fwNum = run[@"FontWeight"];
		BOOL runMono = [run[@"Monospaced"] boolValue] || itemMono;
		CGFloat fs = fsNum.intValue > 0 ? fsNum.floatValue : itemFontSize;
		CGFloat fw = fwNum.floatValue != 0 ? fwNum.floatValue : itemFontWeight;
		NSColor *runColor = MenuetColorFromDict(run[@"Color"]) ?: itemColor;
		NSMutableDictionary *attrs = [NSMutableDictionary new];
		attrs[NSFontAttributeName] = MenuetFont(fs, fw, runMono);
		if (runColor) attrs[NSForegroundColorAttributeName] = runColor;
		[result appendAttributedString:[[NSAttributedString alloc] initWithString:segText
		                                                              attributes:attrs]];
	}
	return result;
}

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
		NSArray *runs = dict[@"Runs"];
		NSString *imageName = dict[@"Image"];
		NSNumber *fontSize = dict[@"FontSize"];
		NSNumber *fontWeight = dict[@"FontWeight"];
		NSDictionary *itemColor = dict[@"Color"];
		BOOL itemMono = [dict[@"Monospaced"] boolValue];
		BOOL state = [dict[@"State"] boolValue];
		BOOL hasChildren = [dict[@"HasChildren"] boolValue];
		BOOL clickable = [dict[@"Clickable"] boolValue];
		if (!item || item.isSeparatorItem) {
			item =
				[self insertItemWithTitle:@"" action:nil keyEquivalent:@"" atIndex:i];
		}
		item.attributedTitle = MenuetBuildAttributedTitle(
		    text, runs, fontSize.floatValue, fontWeight.floatValue,
		    itemColor, itemMono);
		// Shortcut display in the menu (⌘N etc.). The global hotkey
		// registration happens Go-side in buildInternalItem.
		NSDictionary *shortcut = dict[@"Shortcut"];
		if ([shortcut isKindOfClass:[NSDictionary class]]) {
			int kc = [shortcut[@"KeyCode"] intValue];
			uint32_t mods = [shortcut[@"Modifiers"] unsignedIntValue];
			item.keyEquivalent = MenuetKeyEquivalentStringForCode(kc);
			item.keyEquivalentModifierMask = MenuetModifierMaskFromCarbon(mods);
		} else {
			item.keyEquivalent = @"";
			item.keyEquivalentModifierMask = 0;
		}
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
		// Submenu closed (either user moved off to another parent-menu
		// item or the whole tracking session ended). Snapshot the field
		// so the query persists across re-opens. Safe to run on AppKit's
		// open-time view-window cycles too — the field's value at that
		// point is whatever populate: just set it to (either the prior
		// savedQuery or "").
		self.savedQuery = self.field.stringValue;
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
}

- (void)menuDidEndTracking:(NSNotification *)note {
	self.savedQuery = self.field.stringValue;
}

// NSTextFieldDelegate: live filtering as the user types.
- (void)controlTextDidChange:(NSNotification *)note {
	[self applyQuery:self.field.stringValue];
}

// NSTextFieldDelegate: Enter activates the first result; Esc closes the menu.
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
			// Force no truncation at the cell layer — bypasses the cached
			// "drawable width" elision NSMenu falls back to mid-tracking.
			NSMutableParagraphStyle *para = [NSMutableParagraphStyle new];
			para.lineBreakMode = NSLineBreakByClipping;
			newItem.attributedTitle = [[NSAttributedString alloc]
			    initWithString:text
			        attributes:@{NSParagraphStyleAttributeName: para}];
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

	[menu update];

	// NSMenu sizes the popup window's content rect once at first display
	// and doesn't grow it for items inserted mid-tracking, so any result
	// wider than what was there originally gets clipped at the cached
	// cell width. Fix: ask the actual NSMenuItemCell for the cell size it
	// wants for each result item (same code AppKit uses to draw), then
	// grow the menu's minimumWidth to fit the widest.
	//
	// Only grow, never shrink — shrinking mid-tracking is jarring and
	// AppKit doesn't actually contract the popup window after we shrink
	// minimumWidth anyway. The width "ratchets up" to fit whatever's
	// widest in this session.
	CGFloat widestCell = 0;
	for (NSMenuItem *it in menu.itemArray) {
		if (it.tag != MENUET_SEARCH_RESULT_TAG) continue;
		NSMenuItemCell *cell = [[NSMenuItemCell alloc] init];
		cell.menuItem = it;
		CGFloat w = cell.cellSize.width;
		[cell release];
		if (w > widestCell) widestCell = w;
	}
	CGFloat needed = ceil(widestCell);
	if (needed > menu.minimumWidth) {
		menu.minimumWidth = needed;
	}
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
