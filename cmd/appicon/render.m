#import <Cocoa/Cocoa.h>

#import "render.h"

unsigned char *AppIconRenderPNG(const char *letter, double red, double green,
                                double blue, int pixels, int *outLen) {
	if (outLen) *outLen = 0;
	@autoreleasepool {
		// Draw into an offscreen bitmap. Unlike lockFocus, an explicit
		// NSGraphicsContext works headless from a plain terminal (no window
		// server session required).
		NSBitmapImageRep *rep = [[NSBitmapImageRep alloc]
		    initWithBitmapDataPlanes:NULL
		                  pixelsWide:pixels
		                  pixelsHigh:pixels
		               bitsPerSample:8
		             samplesPerPixel:4
		                    hasAlpha:YES
		                    isPlanar:NO
		              colorSpaceName:NSCalibratedRGBColorSpace
		                 bytesPerRow:0
		                bitsPerPixel:0];
		if (!rep) {
			return NULL;
		}
		// cgo compiles this under MRC (no -fobjc-arc), and the surrounding
		// @autoreleasepool only drains autoreleased objects — so hand this
		// alloc/init to the pool, which also covers the early-return paths
		// below. rep stays alive until the pool drains, after the PNG bytes
		// are already copied out.
		[rep autorelease];

		NSGraphicsContext *ctx =
		    [NSGraphicsContext graphicsContextWithBitmapImageRep:rep];
		[NSGraphicsContext saveGraphicsState];
		[NSGraphicsContext setCurrentContext:ctx];

		CGFloat edge = (CGFloat)pixels;
		NSRect rect = NSMakeRect(0, 0, edge, edge);

		// ~22% corner radius approximates the macOS app-icon squircle.
		CGFloat radius = edge * 0.22;
		NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:rect
		                                                    xRadius:radius
		                                                    yRadius:radius];
		NSColor *fill = [NSColor colorWithSRGBRed:red
		                                    green:green
		                                     blue:blue
		                                    alpha:1.0];
		[fill setFill];
		[path fill];

		NSString *text = [NSString stringWithUTF8String:(letter ?: "")];
		NSDictionary *attrs = @{
			NSFontAttributeName : [NSFont boldSystemFontOfSize:edge * 0.55],
			NSForegroundColorAttributeName : [NSColor whiteColor],
		};
		NSSize textSize = [text sizeWithAttributes:attrs];
		NSPoint origin = NSMakePoint((edge - textSize.width) / 2.0,
		                             (edge - textSize.height) / 2.0);
		[text drawAtPoint:origin withAttributes:attrs];

		[NSGraphicsContext restoreGraphicsState];

		NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG
		                               properties:@{}];
		if (!png) {
			return NULL;
		}
		NSUInteger len = png.length;
		unsigned char *buf = malloc(len);
		if (!buf) {
			return NULL;
		}
		memcpy(buf, png.bytes, len);
		if (outLen) *outLen = (int)len;
		return buf;
	}
}
