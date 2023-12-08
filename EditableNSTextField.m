#import "EditableNSTextField.h"

// https://blog.kulman.sk/making-copy-paste-work-with-nstextfield/

@implementation EditableNSTextField

NSUInteger commandKey = NSEventModifierFlagCommand;
NSUInteger commandShiftKey = NSEventModifierFlagCommand | NSEventModifierFlagShift;

- (BOOL)performKeyEquivalent:(NSEvent *)event {
    if (event.type == NSEventTypeKeyDown) {
        if ((event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask) == commandKey) {
            NSString *key = event.charactersIgnoringModifiers;
            if ([key isEqualToString:@"x"]) {
                return [NSApp sendAction:@selector(cut:) to:nil from:self];
            } else if ([key isEqualToString:@"c"]) {
                return [NSApp sendAction:@selector(copy:) to:nil from:self];
            } else if ([key isEqualToString:@"v"]) {
                return [NSApp sendAction:@selector(paste:) to:nil from:self];
            } else if ([key isEqualToString:@"z"]) {
                return [NSApp sendAction:@selector(undo:) to:nil from:self];
            } else if ([key isEqualToString:@"a"]) {
                return [NSApp sendAction:@selector(selectAll:) to:nil from:self];
            }
        } else if ((event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask) == commandShiftKey) {
            if ([event.charactersIgnoringModifiers isEqualToString:@"Z"]) {
                return [NSApp sendAction:@selector(redo:) to:nil from:self];
            }
        }
    }
    return [super performKeyEquivalent:event];
}

@end