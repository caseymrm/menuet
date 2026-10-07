package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTabsSerializes(t *testing.T) {
	item := buildInternalItem(Tabs{Labels: []string{"Claude", "AI Gateway"}, Selected: 1, Changed: func(int) {}}, "u1", "p1")
	if item.Type != "tabs" {
		t.Fatalf("Type = %q, want tabs", item.Type)
	}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"Labels":["Claude","AI Gateway"]`, `"Selected":1`, `"Clickable":true`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("%s missing from %s", want, b)
		}
	}
}

func TestTabSelectedCallsChangedWithTheIndex(t *testing.T) {
	a := &Application{visibleMenuItems: map[string]internalItem{}}
	var got []int
	tabs := Tabs{Labels: []string{"a", "b"}, Changed: func(i int) { got = append(got, i) }}
	a.visibleMenuItems["u1"] = buildInternalItem(tabs, "u1", "p1")

	a.tabSelected("u1", 1)
	a.tabSelected("u1", 2)  // out of range: ignored
	a.tabSelected("u1", -1) // out of range: ignored
	a.tabSelected("nope", 0)

	if len(got) != 1 || got[0] != 1 {
		t.Errorf("Changed saw %v, want [1]", got)
	}
}

func TestTabSelectedIgnoresAnItemThatIsNotTabs(t *testing.T) {
	a := &Application{visibleMenuItems: map[string]internalItem{}}
	a.visibleMenuItems["u1"] = buildInternalItem(Regular{Text: "x"}, "u1", "p1")
	a.tabSelected("u1", 0) // must not panic
}

func TestTabsSnapshot(t *testing.T) {
	s := snapshotItem(Tabs{Labels: []string{"a", "b"}, Selected: 1}, 0)
	if s.Type != "tabs" || s.Text != "a | b" {
		t.Errorf("snapshot = %+v, want type tabs, text \"a | b\"", s)
	}
}
