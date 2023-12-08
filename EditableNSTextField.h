#import <Cocoa/Cocoa.h>

@interface EditableNSTextField : NSTextField

- (BOOL)performKeyEquivalent:(NSEvent *)event;

@end