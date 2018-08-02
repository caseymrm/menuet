package menuet

// ItemType represents what type of menu item this is
type ItemType string

const (
	// Regular is a normal item with text and optional callback
	Regular ItemType = ""
	// Separator is a horizontal line
	Separator = "separator"
	// TODO: StartAtLogin, Quit, Image, Spinner, etc
)

// MenuItem represents one item in the dropdown
type MenuItem struct {
	Type ItemType
	Key  string // Only required for the application's Clicked or MenuOpened

	Text       string
	FontSize   int // Default: 14
	FontWeight FontWeight
	State      bool // shows checkmark when set
	Disabled   bool
	Children   bool

	// If set, the application's Clicked is not called for this item
	Clicked func() `json:"-"`
	// If set, the application's MenuOpened is not called for this item
	MenuOpened func() []MenuItem `json:"-"`
}

type internalItem struct {
	Unique       string
	ParentUnique string

	MenuItem
}
