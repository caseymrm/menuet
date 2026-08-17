// Command appicon renders a fallback macOS app icon and produces a .icns
// file end-to-end. It draws a rounded-rect background in a color derived from
// the app name (or an override) with the app's first letter centered in white.
//
// Usage:
//
//	appicon -name "Wishy Watchy" -o path/to/icon.icns [-color "#4A90D9"]
//
// The menuet.mk build uses it to give an unbranded app a distinct icon
// without shipping any image assets.
package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import "render.h"
*/
import "C"

import (
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unsafe"
)

// rgb holds an sRGB color with components in [0,1].
type rgb struct {
	r, g, b float64
}

// iconSize pairs an .iconset filename with the pixel edge to render for it.
// Several filenames share a pixel size (a @2x variant equals the next size's
// @1x), so rendered PNGs are reused by pixel size.
type iconSize struct {
	name   string
	pixels int
}

var iconSizes = []iconSize{
	{"icon_16x16.png", 16},
	{"icon_16x16@2x.png", 32},
	{"icon_32x32.png", 32},
	{"icon_32x32@2x.png", 64},
	{"icon_128x128.png", 128},
	{"icon_128x128@2x.png", 256},
	{"icon_256x256.png", 256},
	{"icon_256x256@2x.png", 512},
	{"icon_512x512.png", 512},
	{"icon_512x512@2x.png", 1024},
}

func main() {
	log.SetFlags(0)
	name := flag.String("name", "", "application name (required)")
	out := flag.String("o", "", "output .icns path (required)")
	colorHex := flag.String("color", "", "optional background color override, e.g. #4A90D9")
	flag.Parse()

	if *name == "" {
		log.Fatal("appicon: -name is required")
	}
	if *out == "" {
		log.Fatal("appicon: -o is required")
	}

	letter := firstLetter(*name)

	color := deriveColor(*name)
	if *colorHex != "" {
		c, err := parseHexColor(*colorHex)
		if err != nil {
			log.Fatalf("appicon: %v", err)
		}
		color = c
	}

	if err := generate(letter, color, *out); err != nil {
		log.Fatalf("appicon: %v", err)
	}
}

// firstLetter returns the uppercased first non-space rune of the name. It
// falls back to "?" when the name has no usable rune (empty or all whitespace),
// so a stray leading space can't produce a blank icon.
func firstLetter(name string) string {
	for _, r := range name {
		if unicode.IsSpace(r) {
			continue
		}
		return string(unicode.ToUpper(r))
	}
	return "?"
}

// deriveColor maps the full app name to a stable, readable color. It hashes
// the name with FNV-1a for the hue, then fixes saturation and brightness so
// white text stays legible on any resulting background.
func deriveColor(name string) rgb {
	h := fnv.New32a()
	h.Write([]byte(name))
	hue := float64(h.Sum32()%360) / 360.0
	return hsvToRGB(hue, 0.55, 0.75)
}

// hsvToRGB converts HSV (each in [0,1]) to sRGB (each in [0,1]).
func hsvToRGB(h, s, v float64) rgb {
	if s == 0 {
		return rgb{v, v, v}
	}
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	switch i % 6 {
	case 0:
		return rgb{v, t, p}
	case 1:
		return rgb{q, v, p}
	case 2:
		return rgb{p, v, t}
	case 3:
		return rgb{p, q, v}
	case 4:
		return rgb{t, p, v}
	default:
		return rgb{v, p, q}
	}
}

// parseHexColor parses "#RRGGBB" or "RRGGBB" into an sRGB color.
func parseHexColor(s string) (rgb, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return rgb{}, fmt.Errorf("invalid color %q: want #RRGGBB", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return rgb{}, fmt.Errorf("invalid color %q: %v", s, err)
	}
	return rgb{
		r: float64((v>>16)&0xff) / 255.0,
		g: float64((v>>8)&0xff) / 255.0,
		b: float64(v&0xff) / 255.0,
	}, nil
}

// generate renders every icon size into a temporary .iconset directory and
// runs iconutil to produce the .icns file at out.
func generate(letter string, color rgb, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	iconset, err := os.MkdirTemp("", "appicon-*.iconset")
	if err != nil {
		return fmt.Errorf("creating iconset dir: %w", err)
	}
	defer os.RemoveAll(iconset)

	// Cache PNGs by pixel size — several .iconset entries share a size.
	pngByPixels := map[int][]byte{}
	for _, sz := range iconSizes {
		png, ok := pngByPixels[sz.pixels]
		if !ok {
			png, err = renderPNG(letter, color, sz.pixels)
			if err != nil {
				return fmt.Errorf("rendering %dpx: %w", sz.pixels, err)
			}
			pngByPixels[sz.pixels] = png
		}
		if err := os.WriteFile(filepath.Join(iconset, sz.name), png, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", sz.name, err)
		}
	}

	cmd := exec.Command("iconutil", "-c", "icns", "-o", out, iconset)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iconutil: %v: %s", err, output)
	}
	return nil
}

// renderPNG calls the AppKit renderer and returns the PNG bytes.
func renderPNG(letter string, color rgb, pixels int) ([]byte, error) {
	cLetter := C.CString(letter)
	defer C.free(unsafe.Pointer(cLetter))

	var length C.int
	buf := C.AppIconRenderPNG(cLetter, C.double(color.r), C.double(color.g),
		C.double(color.b), C.int(pixels), &length)
	if buf == nil {
		return nil, fmt.Errorf("renderer returned no data")
	}
	defer C.free(unsafe.Pointer(buf))

	return C.GoBytes(unsafe.Pointer(buf), length), nil
}
