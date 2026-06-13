package html

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/caseymrm/menuet/v2"
)

// snap is a tiny helper for tests — most assertions are substring checks
// against the output so we don't lock the renderer to exact byte layout.
func snap(state *menuet.MenuState, items ...menuet.SnapshotItem) menuet.Snapshot {
	return menuet.Snapshot{
		Schema: menuet.SnapshotSchema,
		State:  state,
		Items:  items,
	}
}

func TestRendersTitle(t *testing.T) {
	got := Render(snap(&menuet.MenuState{Title: "Hello"}), Options{})
	if !strings.Contains(got, "Hello") {
		t.Errorf("title not rendered: %s", got)
	}
}

func TestRendersRegularItem(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{Type: "regular", Text: "Refresh"}), Options{})
	if !strings.Contains(got, "Refresh") {
		t.Errorf("item text missing: %s", got)
	}
	if !strings.Contains(got, "menu-row") {
		t.Errorf("menu-row class missing: %s", got)
	}
}

func TestRendersSeparator(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{Type: "separator"}), Options{})
	if !strings.Contains(got, "var(--sep)") {
		t.Errorf("separator not rendered: %s", got)
	}
}

func TestRendersStateCheckmark(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{Type: "regular", Text: "Notify", State: true}), Options{})
	if !strings.Contains(got, "✓") {
		t.Errorf("checkmark missing for State=true: %s", got)
	}
}

func TestRendersShortcut(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: "Quit",
		Shortcut: &menuet.Shortcut{KeyCode: 12, Modifiers: menuet.ModCmd}, // ⌘Q
	}), Options{})
	if !strings.Contains(got, "⌘Q") {
		t.Errorf("shortcut missing: %s", got)
	}
}

func TestRendersSubtitle(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: "Lakers vs Warriors",
		Subtitle: []menuet.TextRun{{Text: "Q4 · 2:14 left"}},
	}), Options{})
	if !strings.Contains(got, "Q4 · 2:14 left") {
		t.Errorf("subtitle missing: %s", got)
	}
}

func TestRendersSubmenuChevron(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: "Favorites",
		Children: []menuet.SnapshotItem{{Type: "regular", Text: "Lakers"}},
	}), Options{})
	if !strings.Contains(got, "<polyline") {
		t.Errorf("submenu chevron missing: %s", got)
	}
}

func TestRendersSearchField(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{Type: "search", Text: "Search teams…"}), Options{})
	if !strings.Contains(got, "Search teams…") {
		t.Errorf("search placeholder missing: %s", got)
	}
}

func TestRendersBadge(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular",
		Runs: []menuet.TextRun{
			{Text: "Lakers"}, {Text: "LIVE", Badge: true, Color: menuet.SystemRed},
		},
	}), Options{})
	if !strings.Contains(got, "LIVE") {
		t.Errorf("badge text missing: %s", got)
	}
	if !strings.Contains(got, "border-radius:4px") {
		t.Errorf("badge pill style missing: %s", got)
	}
}

func TestDarkTheme(t *testing.T) {
	got := Render(snap(nil), Options{Theme: ThemeDark})
	if !strings.Contains(got, "rgba(46,46,48,0.74)") {
		t.Errorf("dark panel bg missing: %s", got)
	}
}

func TestAccentOverride(t *testing.T) {
	got := Render(snap(nil), Options{AccentHex: "#ff00aa"})
	if !strings.Contains(got, "#ff00aa") {
		t.Errorf("accent override not applied: %s", got)
	}
}

func TestAccentOverrideRejectsGarbage(t *testing.T) {
	got := Render(snap(nil), Options{AccentHex: `red;background:url(evil)`})
	if strings.Contains(got, "url(evil)") {
		t.Errorf("garbage accent leaked into output: %s", got)
	}
	if !strings.Contains(got, "#007aff") {
		t.Errorf("default accent missing after garbage rejection: %s", got)
	}
}

// --- Safety boundary tests ---
//
// These are the load-bearing tests. The renderer accepts snapshots from
// arbitrary third-party apps, so XSS and CSS-injection vectors via input
// data MUST be rendered inert. If any of these fail, fix the renderer
// before shipping.

func TestEscapesScriptInText(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: `<script>alert(1)</script>`,
	}), Options{})
	if strings.Contains(got, "<script>") {
		t.Fatalf("script tag leaked into output: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("script tag not escaped: %s", got)
	}
}

func TestEscapesScriptInRun(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular",
		Runs: []menuet.TextRun{{Text: `<img src=x onerror=alert(1)>`}},
	}), Options{})
	if strings.Contains(got, "<img src=x") {
		t.Fatalf("img tag leaked: %s", got)
	}
}

func TestEscapesQuotesInText(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: `" onmouseover="alert(1)`,
	}), Options{})
	if strings.Contains(got, `onmouseover="alert`) {
		t.Fatalf("attribute-context injection leaked: %s", got)
	}
}

func TestRejectsUnknownSemanticColor(t *testing.T) {
	// A malicious snapshot tries to smuggle CSS via a Semantic value.
	// colorCSS must drop unknown semantic names entirely so they can
	// never appear in the style attribute.
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular", Text: "x",
		Color: menuet.Color{Semantic: `red;background:url(http://evil)`},
	}), Options{})
	if strings.Contains(got, "url(http://evil)") {
		t.Fatalf("semantic color value leaked into CSS: %s", got)
	}
	if strings.Contains(got, "red;background") {
		t.Fatalf("semantic color string leaked into CSS: %s", got)
	}
}

func TestRejectsAbsurdFontSize(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular",
		Runs: []menuet.TextRun{{Text: "x", FontSize: 9999}},
	}), Options{})
	if strings.Contains(got, "9999px") {
		t.Fatalf("absurd font-size accepted: %s", got)
	}
}

func TestShadowClamped(t *testing.T) {
	got := Render(snap(nil, menuet.SnapshotItem{
		Type: "regular",
		Runs: []menuet.TextRun{{
			Text:   "x",
			Shadow: &menuet.Shadow{Blur: 99999, OffsetX: 99999, OffsetY: 99999, Color: menuet.SystemYellow},
		}},
	}), Options{})
	// We don't assert exact values, just that the absurd ones didn't pass through.
	if strings.Contains(got, "99999") {
		t.Fatalf("shadow values not clamped: %s", got)
	}
	if !strings.Contains(got, "text-shadow:") {
		t.Errorf("shadow not emitted at all: %s", got)
	}
}

func TestRecursionDepthCappedInRenderer(t *testing.T) {
	// We don't directly test menuet's snapshotter here, but the renderer
	// must not crash on a snapshot with deep submenus. Build one 50 deep.
	leaf := menuet.SnapshotItem{Type: "regular", Text: "leaf"}
	for i := 0; i < 50; i++ {
		leaf = menuet.SnapshotItem{Type: "regular", Text: "x", Children: []menuet.SnapshotItem{leaf}}
	}
	// Must not panic.
	_ = Render(snap(nil, leaf), Options{})
}

func TestSchemaMismatchEmitsComment(t *testing.T) {
	s := menuet.Snapshot{Schema: "menuet-snapshot/v999", Items: []menuet.SnapshotItem{
		{Type: "regular", Text: "x"},
	}}
	got := Render(s, Options{})
	if !strings.Contains(got, "unknown schema") {
		t.Errorf("schema mismatch comment missing: %s", got)
	}
	if !strings.Contains(got, ">x<") {
		t.Errorf("items still rendered best-effort: %s", got)
	}
}

func TestRoundTripsThroughJSON(t *testing.T) {
	// End-to-end: encode a snapshot, decode it, render. Ensures the
	// JSON tags are right and nothing gets lost in serialization.
	in := snap(&menuet.MenuState{
		Runs: []menuet.TextRun{
			{Text: "GSW ", FontWeight: menuet.WeightSemibold},
			{Text: "71", Monospaced: true},
		},
	},
		menuet.SnapshotItem{Type: "regular", Text: "Refresh"},
		menuet.SnapshotItem{Type: "separator"},
		menuet.SnapshotItem{Type: "regular", Text: "Quit",
			Shortcut: &menuet.Shortcut{KeyCode: 12, Modifiers: menuet.ModCmd}},
	)
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out menuet.Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	got := Render(out, Options{})
	for _, want := range []string{"GSW", "71", "Refresh", "Quit", "⌘Q"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in round-trip output: %s", want, got)
		}
	}
}
