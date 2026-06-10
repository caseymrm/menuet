package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

// Color.Semantic tests

func TestSemanticColorIsNotZero(t *testing.T) {
	if LabelPrimary.IsZero() {
		t.Error("LabelPrimary should not be IsZero")
	}
	if SystemRed.IsZero() {
		t.Error("SystemRed should not be IsZero")
	}
}

func TestSemanticColorSerializesSemanticField(t *testing.T) {
	b, _ := json.Marshal(LabelSecondary)
	got := string(b)
	if !strings.Contains(got, `"Semantic":"secondaryLabelColor"`) {
		t.Errorf("semantic field missing\nfull: %s", got)
	}
	// And the RGBA fields should be zero/omitted-like
	if strings.Contains(got, `"R":255`) {
		t.Errorf("semantic color shouldn't leak R\nfull: %s", got)
	}
}

func TestSemanticColorRoundTrip(t *testing.T) {
	var c Color
	if err := json.Unmarshal([]byte(`{"Semantic":"systemRedColor"}`), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Semantic != "systemRedColor" {
		t.Errorf("Semantic round-trip failed: got %q", c.Semantic)
	}
}

// MenuState.Runs tests

func TestMenuStateRunsSerialize(t *testing.T) {
	s := &MenuState{
		Title: "fallback",
		Runs: []TextRun{
			{Text: "GSW", FontWeight: WeightSemibold},
			{Text: " 7:30pm", Color: LabelSecondary},
		},
	}
	b, _ := json.Marshal(s)
	got := string(b)
	for _, want := range []string{
		`"Title":"fallback"`,
		`"Runs":[`,
		`"Text":"GSW"`,
		`"FontWeight":0.3`,
		`"Semantic":"secondaryLabelColor"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("MenuState JSON missing %q\nfull: %s", want, got)
		}
	}
}

// Regular.Subtitle tests

func TestRegularSubtitleSerializes(t *testing.T) {
	item := buildInternalItem(
		Regular{
			Text: "GSW 71 – 68 MIN",
			Subtitle: []TextRun{
				{Text: "NBA · Q3 5:42"},
			},
		},
		"u", "p",
	)
	b, _ := json.Marshal(item)
	got := string(b)
	if !strings.Contains(got, `"Subtitle":[`) {
		t.Errorf("Subtitle missing\nfull: %s", got)
	}
	if !strings.Contains(got, `"Text":"NBA · Q3 5:42"`) {
		t.Errorf("subtitle text missing\nfull: %s", got)
	}
}

func TestRegularWithoutSubtitleOmitsKey(t *testing.T) {
	item := buildInternalItem(Regular{Text: "hello"}, "u", "p")
	b, _ := json.Marshal(item)
	if strings.Contains(string(b), `"Subtitle":`) {
		t.Errorf("empty Subtitle should be omitted; got: %s", b)
	}
}

// TextRun.Badge tests

func TestBadgeRunSerializes(t *testing.T) {
	r := TextRun{Text: "LIVE", Color: SystemRed, Badge: true}
	b, _ := json.Marshal(r)
	got := string(b)
	if !strings.Contains(got, `"Badge":true`) {
		t.Errorf("Badge field missing\nfull: %s", got)
	}
	if !strings.Contains(got, `"Semantic":"systemRedColor"`) {
		t.Errorf("Badge color should still serialize\nfull: %s", got)
	}
}
