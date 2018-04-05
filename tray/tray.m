#import <Cocoa/Cocoa.h>

#import "tray.h"

@interface CantSleepDelegate : NSObject <NSApplicationDelegate>

@end

NSStatusItem *_statusItem;

void itemClicked(const char *);

void setMenuState(NSString *title, NSArray *processes) {
  dispatch_async(dispatch_get_main_queue(), ^{
    CantSleepDelegate *delegate = NSApplication.sharedApplication.delegate;
    _statusItem.title = title;
    if ([processes isKindOfClass:[NSArray class]]) {
      NSMenu *menu = [NSMenu new];
      for (int i = 0; i < processes.count; i++) {
        NSDictionary *dict = [processes objectAtIndex:i];
        NSString *text = dict[@"Text"];
        if ([text isEqualTo:@"---"]) {
          [menu addItem:[NSMenuItem separatorItem]];
          continue;
        }
        NSString *callback = dict[@"Callback"];
        if (callback == nil || callback.length == 0) {
          [menu addItemWithTitle:text action:nil keyEquivalent:@""];
        } else {
          NSMenuItem *item = [menu addItemWithTitle:text
                                             action:@selector(press:)
                                      keyEquivalent:@""];
          item.target = delegate;
          item.representedObject = callback;
        }
      }
      [menu addItem:[NSMenuItem separatorItem]];
      [menu addItemWithTitle:@"Quit Can't Sleep"
                      action:@selector(terminate:)
               keyEquivalent:@""];
      _statusItem.menu = menu;
    }
  });
}

void setState(const char *jsonString) {
  NSDictionary *jsonDict = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  setMenuState(jsonDict[@"Title"], jsonDict[@"Items"]);
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

@end
