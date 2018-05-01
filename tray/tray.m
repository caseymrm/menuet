#import <Cocoa/Cocoa.h>

#import "tray.h"

@interface CantSleepDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>

@end

NSStatusItem *_statusItem;

void itemClicked(const char *);
void alertClicked(int);
const char *menuOpened();

void addItemsToMenu(NSMenu *menu, NSArray *items, CantSleepDelegate *delegate) {
  for (int i = 0; i < items.count; i++) {
    NSDictionary *dict = [items objectAtIndex:i];
    NSString *text = dict[@"Text"];
    if ([text isEqualTo:@"---"]) {
      [menu addItem:[NSMenuItem separatorItem]];
      continue;
    }
    NSString *callback = dict[@"Callback"];
    NSMenuItem *item;
    if (callback == nil || callback.length == 0) {
      item = [menu addItemWithTitle:text action:nil keyEquivalent:@""];
    } else {
      item = [menu addItemWithTitle:text
                             action:@selector(press:)
                      keyEquivalent:@""];
      item.target = delegate;
      item.representedObject = callback;
    }
    NSNumber *state = dict[@"State"];
    if ([state isEqualTo:[NSNumber numberWithBool:true]]) {
      item.state = NSOnState;
    }
    NSArray *children = dict[@"Children"];
    if (![children isEqualTo:NSNull.null] && children.count > 0) {
      item.submenu = [NSMenu new];
      addItemsToMenu(item.submenu, children, delegate);
    }
  }
}

void setItems(NSArray *items) {
  CantSleepDelegate *delegate =
      (CantSleepDelegate *)NSApplication.sharedApplication.delegate;
  if (_statusItem.menu) {
    [_statusItem.menu removeAllItems];
  } else {
    _statusItem.menu = [NSMenu new];
    _statusItem.menu.delegate = delegate;
  }
  addItemsToMenu(_statusItem.menu, items, delegate);
  if (items.count > 0) {
    [_statusItem.menu addItem:[NSMenuItem separatorItem]];
  }
  [_statusItem.menu addItemWithTitle:@"Quit"
                              action:@selector(terminate:)
                       keyEquivalent:@""];
}

void setState(const char *jsonString) {
  NSDictionary *state = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  dispatch_async(dispatch_get_main_queue(), ^{
    _statusItem.title = state[@"Title"];
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
    alertClicked(resp - 1000);
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
