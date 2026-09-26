package menuet

import "testing"

func TestStaticOnlyAppliesToInertRows(t *testing.T) {
	cases := []struct {
		name string
		item Regular
		want bool
	}{
		{"plain", Regular{Text: "a"}, false},
		{"static", Regular{Text: "a", Static: true}, true},
		{"static with Clicked", Regular{Text: "a", Static: true, Clicked: func() {}}, false},
		{"static with Children", Regular{Text: "a", Static: true, Children: func() []MenuItem { return nil }}, false},
	}
	for _, c := range cases {
		if got := buildInternalItem(c.item, "u", "p").Static; got != c.want {
			t.Errorf("%s: internal Static = %v, want %v", c.name, got, c.want)
		}
		if got := snapshotItem(c.item, 0).Static; got != c.want {
			t.Errorf("%s: snapshot Static = %v, want %v", c.name, got, c.want)
		}
	}
}
