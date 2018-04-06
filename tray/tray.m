#import <Cocoa/Cocoa.h>

#import "tray.h"

@interface CantSleepDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>

@end

NSStatusItem *_statusItem;

void itemClicked(const char *);
const char *menuOpened();

void setItems(NSArray *processes) {
  CantSleepDelegate *delegate =
      (CantSleepDelegate *)NSApplication.sharedApplication.delegate;
  if (_statusItem.menu) {
    [_statusItem.menu removeAllItems];
  } else {
    _statusItem.menu = [NSMenu new];
    [_statusItem.menu setDelegate: delegate];
  }
  for (int i = 0; i < processes.count; i++) {
    NSDictionary *dict = [processes objectAtIndex:i];
    NSString *text = dict[@"Text"];
    if ([text isEqualTo:@"---"]) {
      [_statusItem.menu addItem:[NSMenuItem separatorItem]];
      continue;
    }
    NSString *callback = dict[@"Callback"];
    if (callback == nil || callback.length == 0) {
      [_statusItem.menu addItemWithTitle:text action:nil keyEquivalent:@""];
    } else {
      NSMenuItem *item = [_statusItem.menu addItemWithTitle:text
                                                     action:@selector(press:)
                                              keyEquivalent:@""];
      item.target = delegate;
      item.representedObject = callback;
    }
  }
  [_statusItem.menu addItem:[NSMenuItem separatorItem]];
  [_statusItem.menu addItemWithTitle:@"Quit Can't Sleep"
                              action:@selector(terminate:)
                       keyEquivalent:@""];
}

void setMenuState(NSDictionary *state) {
  dispatch_async(dispatch_get_main_queue(), ^{
    _statusItem.title = state[@"Title"];
    NSArray *processes = state[@"Items"];
    if ([processes isKindOfClass:[NSArray class]]) {
      setItems(processes);
    }
  });
}

void setState(const char *jsonString) {
  NSDictionary *jsonDict = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  setMenuState(jsonDict);
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
  NSArray *processes = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:str]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  setItems(processes);
  free((char *)str);
}
@end
