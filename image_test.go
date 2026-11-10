package menuet

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestImageWithDataSerializes(t *testing.T) {
	// Not a real PNG — buildInternalItem doesn't decode, it just forwards.
	raw := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a}
	item := buildInternalItem(Image{Data: raw, MaxWidth: 480, MaxHeight: 360}, "u1", "p1")

	if item.Type != "image" {
		t.Fatalf("Type = %q, want image", item.Type)
	}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// encoding/json base64s a []byte; the ObjC side decodes it back.
	want := base64.StdEncoding.EncodeToString(raw)
	if !strings.Contains(string(b), want) {
		t.Errorf("ImageData not base64-encoded into JSON: %s", b)
	}
	if !strings.Contains(string(b), `"MaxWidth":480`) ||
		!strings.Contains(string(b), `"MaxHeight":360`) {
		t.Errorf("max bounds missing: %s", b)
	}
}

func TestImageWithPathSerializes(t *testing.T) {
	item := buildInternalItem(Image{Path: "/tmp/shot.png"}, "u1", "p1")
	if item.ImagePath != "/tmp/shot.png" {
		t.Errorf("ImagePath = %q", item.ImagePath)
	}
	b, _ := json.Marshal(item)
	// A path-sourced image must not smuggle an empty ImageData key across the
	// bridge — omitempty keeps the payload lean.
	if strings.Contains(string(b), `"ImageData"`) {
		t.Errorf("empty ImageData should be omitted: %s", b)
	}
}

func TestImageClickableOnlyWhenClickedSet(t *testing.T) {
	still := buildInternalItem(Image{Data: []byte{1}}, "u1", "p1")
	if still.Clickable {
		t.Error("a decorative Image must not be Clickable")
	}
	live := buildInternalItem(Image{Data: []byte{1}, Clicked: func() {}}, "u2", "p1")
	if !live.Clickable {
		t.Error("an Image with Clicked must be Clickable")
	}
}

func TestImageClickedAtIsPositional(t *testing.T) {
	item := buildInternalItem(Image{Data: []byte{1}, ClickedAt: func(x, y float64) {}}, "u1", "p1")
	if !item.Clickable || !item.Positional {
		t.Errorf("Clickable, Positional = %v, %v, want true, true", item.Clickable, item.Positional)
	}
	plain := buildInternalItem(Image{Data: []byte{1}, Clicked: func() {}}, "u2", "p1")
	if plain.Positional {
		t.Error("an Image with only Clicked must not be Positional")
	}
}

func TestImageHoveredIsHoverable(t *testing.T) {
	item := buildInternalItem(Image{Data: []byte{1}, Hovered: func(x, y float64) []byte { return nil }}, "u1", "p1")
	if !item.Hoverable {
		t.Error("an Image with Hovered must be Hoverable")
	}
	if buildInternalItem(Image{Data: []byte{1}}, "u2", "p1").Hoverable {
		t.Error("an Image without Hovered must not be Hoverable")
	}
}

func TestHoveredReturnsPicture(t *testing.T) {
	a := App()
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems["bar"] = buildInternalItem(Image{Data: []byte{1},
		Hovered: func(x, y float64) []byte { return []byte{byte(x * 10), byte(y * 10)} }}, "bar", "p1")
	a.visibleMenuItemsMutex.Unlock()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		delete(a.visibleMenuItems, "bar")
		a.visibleMenuItemsMutex.Unlock()
	}()

	if got := a.hovered("bar", 0.5, 2); string(got) != string([]byte{5, 10}) {
		t.Errorf("hovered = %v, want [5 10]", got)
	}
	if got := a.hovered("missing", 0.5, 0.5); got != nil {
		t.Errorf("hovered on a missing row = %v, want nil", got)
	}
}

func TestClickedAtClampsToPicture(t *testing.T) {
	a := App()
	got := make(chan [2]float64, 1)
	a.visibleMenuItemsMutex.Lock()
	a.visibleMenuItems["seek"] = buildInternalItem(Image{Data: []byte{1},
		ClickedAt: func(x, y float64) { got <- [2]float64{x, y} }}, "seek", "p1")
	a.visibleMenuItemsMutex.Unlock()
	defer func() {
		a.visibleMenuItemsMutex.Lock()
		delete(a.visibleMenuItems, "seek")
		a.visibleMenuItemsMutex.Unlock()
	}()

	a.clickedAt("seek", -0.1, 1.2)
	select {
	case p := <-got:
		if p != [2]float64{0, 1} {
			t.Errorf("ClickedAt got %v, want [0 1]", p)
		}
	case <-time.After(time.Second):
		t.Fatal("ClickedAt was not invoked within 1s")
	}
}

// Default bounds are substituted once here in Go — every consumer (ObjC
// bridge, snapshots) sees concrete values, so the defaults live in exactly
// one place. Per-axis: an explicit value on one axis keeps the default on
// the other.
func TestImageDefaultBoundsSubstituted(t *testing.T) {
	item := buildInternalItem(Image{Data: []byte{1}}, "u1", "p1")
	if item.MaxWidth != DefaultMaxImageWidth || item.MaxHeight != DefaultMaxImageHeight {
		t.Errorf("zero bounds should become defaults, got %dx%d",
			item.MaxWidth, item.MaxHeight)
	}
	item = buildInternalItem(Image{Data: []byte{1}, MaxWidth: 200}, "u2", "p1")
	if item.MaxWidth != 200 || item.MaxHeight != DefaultMaxImageHeight {
		t.Errorf("explicit width should keep default height, got %dx%d",
			item.MaxWidth, item.MaxHeight)
	}
}

func TestImageClickRoutesToCallback(t *testing.T) {
	fired := make(chan struct{}, 1)
	a := &Application{visibleMenuItems: make(map[string]internalItem)}
	img := Image{Data: []byte{1}, Clicked: func() { fired <- struct{}{} }}
	a.visibleMenuItems["u1"] = buildInternalItem(img, "u1", "root")

	a.clicked("u1")
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("clicking an Image did not invoke its Clicked callback")
	}
}

func TestClickedAtTakesPrecedenceOverClicked(t *testing.T) {
	at := make(chan struct{}, 1)
	plain := make(chan struct{}, 1)
	img := Image{Data: []byte{1},
		Clicked:   func() { plain <- struct{}{} },
		ClickedAt: func(x, y float64) { at <- struct{}{} }}
	item := buildInternalItem(img, "u1", "root")
	if !item.Clickable || !item.Positional {
		t.Fatalf("Clickable, Positional = %v, %v, want true, true", item.Clickable, item.Positional)
	}
	a := &Application{visibleMenuItems: map[string]internalItem{"u1": item}}

	a.clickedAt("u1", 0.5, 0.5)
	select {
	case <-at:
	case <-time.After(time.Second):
		t.Fatal("ClickedAt was not invoked within 1s")
	}
	select {
	case <-plain:
		t.Error("clickedAt must not invoke Clicked")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestClickedAtOnMissingRow(t *testing.T) {
	a := &Application{visibleMenuItems: make(map[string]internalItem)}
	a.clickedAt("missing", 0.5, 0.5) // logs and returns; must not panic
}

func TestHoveredEmptyIsNil(t *testing.T) {
	for name, pic := range map[string][]byte{"nil": nil, "empty": {}} {
		img := Image{Data: []byte{1}, Hovered: func(x, y float64) []byte { return pic }}
		a := &Application{visibleMenuItems: map[string]internalItem{
			"bar": buildInternalItem(img, "bar", "root")}}
		if got := a.hovered("bar", 0.5, 0.5); got != nil {
			t.Errorf("%s: hovered = %#v, want nil", name, got)
		}
	}
}

func TestHoveredOnRowWithoutHovered(t *testing.T) {
	a := &Application{visibleMenuItems: map[string]internalItem{
		"img": buildInternalItem(Image{Data: []byte{1}}, "img", "root"),
		"reg": buildInternalItem(Regular{Text: "x"}, "reg", "root"),
	}}
	for _, u := range []string{"img", "reg"} {
		if got := a.hovered(u, 0.5, 0.5); got != nil {
			t.Errorf("hovered(%q) = %v, want nil", u, got)
		}
	}
}

func TestUnit(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{0, 0},
		{1, 1},
		{0.25, 0.25},
		{-0.5, 0},
		{1.5, 1},
		{math.Inf(-1), 0},
		{math.Inf(1), 1},
		{math.NaN(), 0},
	} {
		if got := unit(c.in); got != c.want {
			t.Errorf("unit(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
