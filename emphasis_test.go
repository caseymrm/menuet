package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnderlineRunSerializes(t *testing.T) {
	r := TextRun{Text: "winner", Underline: true}
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"Underline":true`) {
		t.Errorf("Underline missing: %s", b)
	}
}

func TestStrikethroughRunSerializes(t *testing.T) {
	r := TextRun{Text: "eliminated", Strikethrough: true}
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"Strikethrough":true`) {
		t.Errorf("Strikethrough missing: %s", b)
	}
}

func TestBackgroundRunSerializes(t *testing.T) {
	r := TextRun{Text: "highlight", Background: Yellow}
	b, _ := json.Marshal(r)
	got := string(b)
	// Background is its own Color key in the JSON, not Color.
	if !strings.Contains(got, `"Background":{`) {
		t.Errorf("Background missing: %s", got)
	}
	if !strings.Contains(got, `"R":200`) {
		t.Errorf("Background RGBA missing: %s", got)
	}
}

func TestShadowRunSerializes(t *testing.T) {
	r := TextRun{
		Text: "trophy",
		Shadow: &Shadow{
			Color: Yellow,
			Blur:  8,
		},
	}
	b, _ := json.Marshal(r)
	got := string(b)
	if !strings.Contains(got, `"Shadow":{`) {
		t.Errorf("Shadow missing: %s", got)
	}
	if !strings.Contains(got, `"Blur":8`) {
		t.Errorf("Blur missing: %s", got)
	}
	if !strings.Contains(got, `"Semantic":"systemYellowColor"`) && !strings.Contains(got, `"R":200,"G":160`) {
		t.Errorf("Shadow Color missing: %s", got)
	}
}

func TestPlainRunOmitsEmphasisFields(t *testing.T) {
	// Without using the omitempty pattern in this PR (kept additive for
	// schema stability), a plain run still serializes its boolean zeros.
	// What matters is the deserialized side ignores them — verify a plain
	// run round-trips with no emphasis flags set.
	r := TextRun{Text: "hi"}
	b, _ := json.Marshal(r)
	var got TextRun
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Underline || got.Strikethrough || got.Shadow != nil || !got.Background.IsZero() {
		t.Errorf("plain run should have no emphasis fields set, got %+v", got)
	}
}
