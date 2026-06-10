package menuet

// Color is an RGBA color for menu-item text. The zero value (all four
// components zero) means "use the system default color" — i.e. whatever
// color the menu item would normally render in (which adapts to dark vs.
// light mode automatically).
type Color struct {
	R, G, B, A uint8
}

// IsZero reports whether c is the zero value. A zero-value Color is
// interpreted as "use the system default" rather than fully transparent
// black. To render fully transparent black, set A explicitly to a tiny
// non-zero value.
func (c Color) IsZero() bool { return c == Color{} }

// Named colors callers can use without constructing one manually. Choose
// readable defaults; users can pick custom RGBA when they need to.
var (
	Red    = Color{R: 220, G: 50, B: 50, A: 255}
	Green  = Color{R: 30, G: 160, B: 60, A: 255}
	Yellow = Color{R: 200, G: 160, B: 0, A: 255}
	Blue   = Color{R: 0, G: 110, B: 200, A: 255}
	Gray   = Color{R: 128, G: 128, B: 128, A: 255}
)

// TextRun is one segment of a styled menu-item title. Multiple runs are
// concatenated to form the full title with per-segment styling — see
// Regular.Runs. The zero value of each style field means "inherit" so a
// run can change just one attribute (e.g. only Color) without disturbing
// the others.
type TextRun struct {
	Text       string
	Color      Color      // zero = system default
	FontSize   int        // 0 = inherit from item
	FontWeight FontWeight // 0 = default
	Monospaced bool       // true = system monospace font
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
