#import "NSImage+Resize.h"

@implementation NSImage (Resize)

- (NSImage *)imageWithHeight:(CGFloat)height {
  NSImage *image = self;
  if (![image isValid]) {
    NSLog(@"Can't resize invalid image");
    return nil;
  }
  NSSize newSize =
      NSMakeSize(image.size.width * height / image.size.height, height);
  NSImage *newImage = [[NSImage alloc] initWithSize:newSize];
  [newImage lockFocus];
  [image setSize:newSize];
  [[NSGraphicsContext currentContext]
      setImageInterpolation:NSImageInterpolationDefault];
  [image drawAtPoint:NSZeroPoint
            fromRect:CGRectMake(0, 0, newSize.width, newSize.height)
           operation:NSCompositingOperationCopy
            fraction:1.0];
  [newImage unlockFocus];
  return newImage;
}

+ (NSImage *)imageFromName:(NSString *)name withHeight:(CGFloat)height {
  if (name.length == 0) {
    return nil;
  }
  NSImage *image = nil;
  if ([name hasPrefix:@"http"]) {
    image = [[NSImage alloc] initWithContentsOfURL:[NSURL URLWithString:name]];
  } else {
    image = [NSImage imageNamed:name];
  }
  if (height > 0 && image.size.height > height) {
    image = [image imageWithHeight:height];
  }
  return image;
}

@end