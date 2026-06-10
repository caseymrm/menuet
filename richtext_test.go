package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestColorZeroIsDefault(t *testing.T) {
	if !(Color{}).IsZero() {
		t.Error("Color{} should be zero")
	}
	if (Color{A: 255}).IsZero() {
		t.Error("Color with any non-zero component should NOT be zero")
	}
	if Red.IsZero() {
		t.Error("Red should not be zero")
	}
}

func TestRegularRunsSerializeToJSON(t *testing.T) {
	item := buildInternalItem(
		Regular{
			Text: "fallback",
			Runs: []TextRun{
				{Text: "Status: "},
				{Text: "FAILED", Color: Red, FontWeight: WeightBold},
			},
		},
		"u", "p",
	)
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	// The ObjC side reads "Runs" when non-empty; "Text" is used otherwise.
	for _, want := range []string{
		`"Text":"fallback"`,
		`"Runs":[`,
		`"Text":"Status: "`,
		`"Text":"FAILED"`,
		`"Color":{"R":220,"G":50,"B":50,"A":255}`,
		`"FontWeight":0.4`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, got)
		}
	}
}

func TestRegularWithoutRunsOmitsRunsKey(t *testing.T) {
	item := buildInternalItem(Regular{Text: "hello"}, "u", "p")
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"Runs":`) {
		t.Errorf("empty Runs should be omitted; got: %s", b)
	}
}

func TestRegularRunsPlusItemLevelColor(t *testing.T) {
	// Item-level Color and Monospaced apply to runs that don't override them.
	item := buildInternalItem(
		Regular{
			Color:      Gray,
			Monospaced: true,
			Runs: []TextRun{
				{Text: "inherits gray + mono"},
				{Text: "overrides color", Color: Red},
			},
		},
		"u", "p",
	)
	b, _ := json.Marshal(item)
	got := string(b)
	// Item Color should appear at the top level even when Runs is set —
	// the ObjC side uses it as the inherited default for runs whose own
	// Color is the zero value.
	if !strings.Contains(got, `"Color":{"R":128,"G":128,"B":128,"A":255}`) {
		t.Errorf("item Color missing\nfull: %s", got)
	}
	if !strings.Contains(got, `"Monospaced":true`) {
		t.Errorf("item Monospaced missing\nfull: %s", got)
	}
}
