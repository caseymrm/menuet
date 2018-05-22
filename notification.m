#import <Cocoa/Cocoa.h>

#import "notification.h"

void showNotification(const char *jsonString) {
  NSDictionary *jsonDict = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:jsonString]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:nil];
  NSUserNotification *notification = [NSUserNotification new];
  notification.title = jsonDict[@"Title"];
  notification.subtitle = jsonDict[@"Subtitle"];
  notification.informativeText = jsonDict[@"Message"];
  dispatch_async(dispatch_get_main_queue(), ^{
    NSUserNotificationCenter *center =
        [NSUserNotificationCenter defaultUserNotificationCenter];
    [center deliverNotification:notification];
  });
}
