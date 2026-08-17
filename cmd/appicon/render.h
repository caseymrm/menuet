#ifndef __APPICON_RENDER_H__
#define __APPICON_RENDER_H__

// AppIconRenderPNG draws a rounded-rect app icon at the given pixel size: a
// solid background in the supplied sRGB color (components 0..1) with the
// letter centered in white bold system font. It returns malloc'd PNG bytes
// and writes the length to outLen. The caller must free the returned buffer.
// Returns NULL on failure (outLen set to 0).
unsigned char *AppIconRenderPNG(const char *letter, double red, double green,
                                double blue, int pixels, int *outLen);

#endif
