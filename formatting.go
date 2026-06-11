package menuet

// Color is a color for menu-item text. The zero value means "use the
// system default" — whatever color the menu item would normally render
// in (adapts to dark vs. light mode automatically).
//
// Two ways to specify a color:
//   - RGBA via R, G, B, A. Fixed pixel values. Does not adapt to
//     appearance — usually fine for branded colors that need to match
//     across light and dark mode.
//   - Semantic: a name like "labelColor" or "systemRed" that the ObjC
//     bridge resolves to an AppKit dynamic NSColor at draw time, so the
//     value shifts with appearance automatically. Prefer this for
//     hierarchy and accent colors.
//
// When Semantic is non-empty it takes precedence over RGBA.
type Color struct {
	R, G, B, A uint8
	Semantic   string `json:",omitempty"`
}

// IsZero reports whether c is the zero value.
func (c Color) IsZero() bool { return c == Color{} }

// Fixed-RGBA named colors. These don't adapt to dark / light mode; for
// hierarchy and accent colors that should adapt, use the semantic
// constants (LabelPrimary etc.) below.
var (
	Red    = Color{R: 220, G: 50, B: 50, A: 255}
	Green  = Color{R: 30, G: 160, B: 60, A: 255}
	Yellow = Color{R: 200, G: 160, B: 0, A: 255}
	Blue   = Color{R: 0, G: 110, B: 200, A: 255}
	Gray   = Color{R: 128, G: 128, B: 128, A: 255}
)

// Semantic colors map onto AppKit dynamic NSColors and re-resolve per
// appearance at draw time. Use these for text hierarchy and accents
// that should adapt across light and dark mode.
var (
	LabelPrimary    = Color{Semantic: "labelColor"}           // default text
	LabelSecondary  = Color{Semantic: "secondaryLabelColor"}  // less prominent
	LabelTertiary   = Color{Semantic: "tertiaryLabelColor"}   // metadata / fine print
	LabelQuaternary = Color{Semantic: "quaternaryLabelColor"} // very faint, e.g. spoiler veil

	SystemRed    = Color{Semantic: "systemRedColor"}
	SystemGreen  = Color{Semantic: "systemGreenColor"}
	SystemYellow = Color{Semantic: "systemYellowColor"}
	SystemBlue   = Color{Semantic: "systemBlueColor"}
	SystemOrange = Color{Semantic: "systemOrangeColor"}
	SystemPurple = Color{Semantic: "systemPurpleColor"}
	SystemPink   = Color{Semantic: "systemPinkColor"}
	SystemGray   = Color{Semantic: "systemGrayColor"}
	SystemBrown  = Color{Semantic: "systemBrownColor"}
	SystemTeal   = Color{Semantic: "systemTealColor"}
	SystemIndigo = Color{Semantic: "systemIndigoColor"}
	SystemMint   = Color{Semantic: "systemMintColor"}
	SystemCyan   = Color{Semantic: "systemCyanColor"}
)

// TextRun is one segment of a styled menu-item title. Multiple runs are
// concatenated to form the full title with per-segment styling — see
// Regular.Runs. The zero value of each style field means "inherit" so a
// run can change just one attribute (e.g. only Color) without disturbing
// the others.
//
// When Badge is true the run renders as a filled, rounded pill rather
// than text: Color becomes the fill color and Text appears in white-on-
// fill at small size. Useful for "LIVE" / "NEW" pills next to a row.
//
// Underline, Strikethrough, Background, and Shadow are common emphasis
// attributes — useful for marking winners/losers, "marker-pen" style
// highlights, and trophy-glow celebrations.
type TextRun struct {
	Text          string
	Color         Color      // zero = system default
	FontSize      int        // 0 = inherit from item
	FontWeight    FontWeight // 0 = default
	Monospaced    bool       // true = system monospace font
	Badge         bool       // true = render as rounded-pill badge
	Underline     bool       // single underline in the run's foreground color
	Strikethrough bool       // single strike through the text
	Background    Color      // zero = none; non-zero = colored highlight behind text
	Shadow        *Shadow    // nil = no shadow; set for a drop-shadow or glow
}

// Shadow is a drop-shadow or glow rendered behind a TextRun. Set Blur
// alone (with the default zero offset) for a glow effect; set OffsetX
// and OffsetY for a directional drop shadow. Color of zero defaults to
// translucent black at draw time.
type Shadow struct {
	Color   Color
	Blur    float64 // blur radius in points; 0 = sharp
	OffsetX float64 // horizontal offset in points
	OffsetY float64 // vertical offset in points (AppKit: positive = up)
}

// FontWeight represents the weight of the font
type FontWeight float64

const (
	// WeightUltraLight is equivalent to NSFontWeightUltraLight
	WeightUltraLight FontWeight = -0.8
	// WeightThin is equivalent to NSFontWeightThin
	WeightThin = -0.6
	// WeightLight is equivalent to NSFontWeightLight
	WeightLight = -0.4
	// WeightRegular is equivalent to NSFontWeightRegular, and is the default
	WeightRegular = 0
	// WeightMedium is equivalent to NSFontWeightMedium
	WeightMedium = 0.23
	// WeightSemibold is equivalent to NSFontWeightSemibold
	WeightSemibold = 0.3
	// WeightBold is equivalent to NSFontWeightBold
	WeightBold = 0.4
	// WeightHeavy is equivalent to NSFontWeightHeavy
	WeightHeavy = 0.56
	// WeightBlack is equivalent to NSFontWeightBlack
	WeightBlack = 0.62
)
