package html

import (
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/caseymrm/menuet/v2"
)

// renderRun emits one TextRun as a <span> with a whitelisted set of CSS
// properties. inBar tweaks the rendering to fit the menu-bar strip
// (smaller default size, slightly tighter letter-spacing).
//
// This is the only place untrusted data turns into HTML. Every text
// value is HTML-escaped; every color, weight, and shadow is validated
// before being written to the style attribute. The renderer never emits
// arbitrary CSS — only properties from the fixed whitelist below.
func renderRun(b *strings.Builder, run menuet.TextRun, inBar bool) {
	if run.Badge {
		renderBadge(b, run)
		return
	}
	var style strings.Builder
	if c := colorCSS(run.Color); c != "" {
		fmt.Fprintf(&style, "color:%s;", c)
	}
	if w := weightCSS(run.FontWeight); w != "" {
		fmt.Fprintf(&style, "font-weight:%s;", w)
	}
	if run.FontSize > 0 && run.FontSize <= 96 {
		fmt.Fprintf(&style, "font-size:%dpx;", run.FontSize)
	}
	if run.Monospaced {
		style.WriteString("font-family:'IBM Plex Mono',ui-monospace,monospace;letter-spacing:-0.02em;")
	} else if inBar {
		style.WriteString("font-weight:600;")
	}
	if run.Underline || run.Strikethrough {
		var decos []string
		if run.Underline {
			decos = append(decos, "underline")
		}
		if run.Strikethrough {
			decos = append(decos, "line-through")
		}
		fmt.Fprintf(&style, "text-decoration:%s;", strings.Join(decos, " "))
		// When a separate underline/strikethrough color is given, prefer
		// underline's; strikethrough's is honored only if underline isn't
		// set. The two-color case (different colors for both) isn't
		// representable in standard CSS, so we accept the limitation.
		if c := colorCSS(run.UnderlineColor); run.Underline && c != "" {
			fmt.Fprintf(&style, "text-decoration-color:%s;", c)
		} else if c := colorCSS(run.StrikethroughColor); run.Strikethrough && c != "" {
			fmt.Fprintf(&style, "text-decoration-color:%s;", c)
		}
	}
	if c := colorCSS(run.Background); c != "" {
		fmt.Fprintf(&style, "background:%s;padding:0 3px;border-radius:3px;", c)
	}
	if run.Shadow != nil {
		ox := clampFloat(run.Shadow.OffsetX, -32, 32)
		// AppKit's y is "up positive"; CSS's y is "down positive". Flip.
		oy := -clampFloat(run.Shadow.OffsetY, -32, 32)
		blur := clampFloat(run.Shadow.Blur, 0, 64)
		c := colorCSS(run.Shadow.Color)
		if c == "" {
			c = "rgba(0,0,0,0.5)"
		}
		fmt.Fprintf(&style, "text-shadow:%gpx %gpx %gpx %s;", ox, oy, blur, c)
	}
	b.WriteString(`<span style="`)
	b.WriteString(style.String())
	b.WriteString(`">`)
	b.WriteString(stdhtml.EscapeString(run.Text))
	b.WriteString(`</span>`)
}

// renderBadge emits a TextRun marked Badge=true as a filled rounded pill,
// matching the look of the LIVE chip in the menu-bar mockup.
func renderBadge(b *strings.Builder, run menuet.TextRun) {
	bg := colorCSS(run.Color)
	if bg == "" {
		bg = "var(--live)"
	}
	fmt.Fprintf(b,
		`<span style="font-size:9px;font-weight:700;letter-spacing:0.05em;background:%s;color:#fff;padding:1px 5px;border-radius:4px;line-height:1.5;">%s</span>`,
		bg, stdhtml.EscapeString(run.Text))
}

// colorCSS returns a CSS color value for a menuet.Color, or "" if the
// color is the zero value. Semantic names are mapped to their NSColor
// equivalents (and to --accent for the system accent); any unknown
// semantic name is rejected as "" so injected names can't leak through.
func colorCSS(c menuet.Color) string {
	if c.IsZero() {
		return ""
	}
	if c.Semantic != "" {
		v, ok := semanticColors[c.Semantic]
		if !ok {
			return "" // unknown semantic name — drop, don't pass through
		}
		return v
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", c.R, c.G, c.B, float64(c.A)/255.0)
}

// semanticColors maps the menuet.Color.Semantic names to fixed CSS
// values. Keep in lockstep with formatting.go's semantic vars. We do NOT
// pass the Semantic string through to CSS — the whitelist below is the
// safety boundary that prevents `Semantic: "red; background: url(evil)"`
// from being interpreted as raw CSS.
var semanticColors = map[string]string{
	"labelColor":            "var(--text)",
	"secondaryLabelColor":   "var(--text-2)",
	"tertiaryLabelColor":    "var(--text-3)",
	"quaternaryLabelColor":  "var(--text-3)",
	"systemRedColor":        "#ff3b30",
	"systemGreenColor":      "#34c759",
	"systemYellowColor":     "#ffcc00",
	"systemBlueColor":       "var(--accent)",
	"systemOrangeColor":     "#ff9500",
	"systemPurpleColor":     "#af52de",
	"systemPinkColor":       "#ff2d55",
	"systemGrayColor":       "#8e8e93",
	"systemBrownColor":      "#a2845e",
	"systemTealColor":       "#5ac8fa",
	"systemIndigoColor":     "#5856d6",
	"systemMintColor":       "#00c7be",
	"systemCyanColor":       "#32ade6",
}

// weightCSS maps menuet.FontWeight (NSFontWeight scale) to a numeric CSS
// font-weight. Bounds chosen to span the full NS scale; values outside
// the range are dropped to inherit.
func weightCSS(w menuet.FontWeight) string {
	switch {
	case w <= -0.7:
		return "100" // UltraLight
	case w <= -0.5:
		return "200" // Thin
	case w <= -0.3:
		return "300" // Light
	case w < 0.2:
		return "" // Regular = inherit
	case w < 0.27:
		return "500" // Medium
	case w < 0.35:
		return "600" // Semibold
	case w < 0.5:
		return "700" // Bold
	case w < 0.59:
		return "800" // Heavy
	default:
		return "900" // Black
	}
}

// shortcutString formats a Shortcut as the Apple-standard display
// (⌃⌥⇧⌘ then key glyph). Keep the key-code map in sync with
// MenuetKeyEquivalentStringForCode in menuet.m.
func shortcutString(s menuet.Shortcut) string {
	var b strings.Builder
	if s.Modifiers&menuet.ModCtrl != 0 {
		b.WriteString("⌃")
	}
	if s.Modifiers&menuet.ModAlt != 0 {
		b.WriteString("⌥")
	}
	if s.Modifiers&menuet.ModShift != 0 {
		b.WriteString("⇧")
	}
	if s.Modifiers&menuet.ModCmd != 0 {
		b.WriteString("⌘")
	}
	b.WriteString(keyCodeGlyph(s.KeyCode))
	return b.String()
}

// keyCodeGlyph maps a macOS virtual key code to its menu-display glyph.
// Letters are uppercased to match Apple's HIG. Unknown codes render as
// the empty string.
func keyCodeGlyph(code uint16) string {
	switch code {
	case 0:
		return "A"
	case 11:
		return "B"
	case 8:
		return "C"
	case 2:
		return "D"
	case 14:
		return "E"
	case 3:
		return "F"
	case 5:
		return "G"
	case 4:
		return "H"
	case 34:
		return "I"
	case 38:
		return "J"
	case 40:
		return "K"
	case 37:
		return "L"
	case 46:
		return "M"
	case 45:
		return "N"
	case 31:
		return "O"
	case 35:
		return "P"
	case 12:
		return "Q"
	case 15:
		return "R"
	case 1:
		return "S"
	case 17:
		return "T"
	case 32:
		return "U"
	case 9:
		return "V"
	case 13:
		return "W"
	case 7:
		return "X"
	case 16:
		return "Y"
	case 6:
		return "Z"
	case 29:
		return "0"
	case 18:
		return "1"
	case 19:
		return "2"
	case 20:
		return "3"
	case 21:
		return "4"
	case 23:
		return "5"
	case 22:
		return "6"
	case 26:
		return "7"
	case 28:
		return "8"
	case 25:
		return "9"
	case 49:
		return "␣" // space glyph
	case 36:
		return "↩"
	case 48:
		return "⇥"
	case 53:
		return "⎋"
	case 122:
		return "F1"
	case 120:
		return "F2"
	case 99:
		return "F3"
	case 118:
		return "F4"
	case 96:
		return "F5"
	case 97:
		return "F6"
	case 98:
		return "F7"
	case 100:
		return "F8"
	case 101:
		return "F9"
	case 109:
		return "F10"
	case 103:
		return "F11"
	case 111:
		return "F12"
	case 123:
		return "←"
	case 124:
		return "→"
	case 125:
		return "↓"
	case 126:
		return "↑"
	}
	return ""
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// isHexColor reports whether s is "#" + 6 hex digits. Used to validate
// the caller-supplied AccentHex before letting it into CSS.
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !ok {
			return false
		}
	}
	return true
}
