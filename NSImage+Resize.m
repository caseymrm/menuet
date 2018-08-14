#import "NSImage+Resize.h"

@implementation NSImage (Resize)

- (NSImage *)imageWithHeight:(CGFloat)height {
  NSImage *image = self;
  if (![image isValid]) {
    NSLog(@"Can't resize invalid image");
    return nil;
  }
  NSSize newSize = NSMakeSize(image.size.width * height / image.size.height, height);
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

@end