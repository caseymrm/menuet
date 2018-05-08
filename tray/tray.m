#import <Cocoa/Cocoa.h>

#import "tray.h"

@interface CantSleepDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>

@end

NSStatusItem *_statusItem;

void itemClicked(const char *);
void alertClicked(int);
const char *menuOpened();
bool runningAtStartup();
void toggleStartup();

void addItemsToMenu(NSMenu *menu, NSArray *items, CantSleepDelegate *delegate) {
  for (int i = 0; i < items.count; i++) {
    NSDictionary *dict = [items objectAtIndex:i];
    NSString *text = dict[@"Text"];
    if ([text isEqualTo:@"---"]) {
      [menu addItem:[NSMenuItem separatorItem]];
      continue;
    }
    NSString *callback = dict[@"Callback"];
    NSNumber *state = dict[@"State"];
    NSArray *children = dict[@"Children"];
    NSMenuItem *item = nil;
    if (i < menu.numberOfItems) {
      item = [menu itemAtIndex:i];
    }
    if (!item) {
      item = [menu addItemWithTitle:@"" action:nil keyEquivalent:@""];
    }
    item.title = text;
    item.target = delegate;
    if (callback == nil || callback.length == 0) {
      if (![children isEqualTo:NSNull.null] && children.count > 0) {
        item.action = @selector(nop:);
      } else {
        item.action = nil;
      }
      item.representedObject = nil;
    } else {
      item.action = @selector(press:);
      item.representedObject = callback;
    }
    if ([state isEqualTo:[NSNumber numberWithBool:true]]) {
      item.state = NSOnState;
    } else {
      item.state = NSOffState;
    }
    if (![children isEqualTo:NSNull.null] && children.count > 0) {
      if (!item.submenu) {
        item.submenu = [NSMenu new];
      }
      addItemsToMenu(item.submenu, children, delegate);
    } else if (item.submenu) {
      item.submenu = nil;
    }
  }
  while (menu.numberOfItems > items.count) {
    [menu removeItemAtIndex:menu.numberOfItems - 1];
  }
}

void setItems(NSArray *items) {
  CantSleepDelegate *delegate =
      (CantSleepDelegate *)NSApplication.sharedApplication.delegate;
  if (!_statusItem.menu) {
    _statusItem.menu = [NSMenu new];
    _statusItem.menu.delegate = delegate;
  }
  items = [items arrayByAddingObjectsFromArray:@[
    @{@"Text" : @"---"},
    @{@"Text" : @"Run at start up"},
    @{@"Text" : @"Quit"},
  ]];
  addItemsToMenu(_statusItem.menu, items, delegate);

  NSMenuItem *item = [_statusItem.menu itemAtIndex:items.count - 2];
  item.action = @selector(toggleStartup:);
  if (runningAtStartup()) {
    item.state = NSOnState;
  } else {
    item.state = NSOffState;
  }

  item = [_statusItem.menu itemAtIndex:items.count - 1];
  item.target = nil;
  item.action = @selector(terminate:);
}

void setState(const char *jsonString) {
  NSDictionary *state = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  dispatch_async(dispatch_get_main_queue(), ^{
    _statusItem.button.title = state[@"Title"];
    _statusItem.button.image = [NSImage imageNamed:state[@"Image"]];
    NSArray *items = state[@"Items"];
    if ([items isKindOfClass:[NSArray class]]) {
      setItems(items);
    } else {
      setItems(@[]);
    }
  });
}

void showAlert(const char *jsonString) {
  NSDictionary *jsonDict = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  NSAlert *alert = [NSAlert new];
  // alert.alertStyle = NSAlertStyle.CriticalAlertStyle;
  alert.messageText = jsonDict[@"MessageText"];
  alert.informativeText = jsonDict[@"InformativeText"];
  NSArray *buttons = jsonDict[@"Buttons"];
  for (NSString *label in buttons) {
    [alert addButtonWithTitle:label];
  }
  dispatch_async(dispatch_get_main_queue(), ^{
    NSInteger resp = [alert runModal];
    alertClicked(resp - NSAlertFirstButtonReturn);
  });
}

void createAndRunApplication() {
  [NSAutoreleasePool new];
  NSApplication *a = NSApplication.sharedApplication;
  [a setActivationPolicy:NSApplicationActivationPolicyAccessory];
  _statusItem = [[NSStatusBar systemStatusBar]
      statusItemWithLength:NSVariableStatusItemLength];
  CantSleepDelegate *p = [CantSleepDelegate new];
  [a setDelegate:p];
  [a run];
}

@implementation CantSleepDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:
    (NSApplication *)sender {
  return NSTerminateNow;
}

- (void)press:(id)sender {
  NSString *callback = [sender representedObject];
  itemClicked(callback.UTF8String);
}

- (void)toggleStartup:(id)sender {
  toggleStartup();
}

- (void)nop:(id)sender {
}

- (void)menuWillOpen:(NSMenu *)menu {
  const char *str = menuOpened();
  if (str == NULL) {
    return;
  }
  NSArray *items = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:str]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  setItems(items);
  free((char *)str);
}
@end
