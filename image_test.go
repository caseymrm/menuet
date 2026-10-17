package menuet

import (
	"encoding/base64"
	"encoding/json"
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
