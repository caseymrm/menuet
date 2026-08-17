package main

import "testing"

func TestFirstLetter(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Wishy Watchy", "W"},
		{"hello world", "H"},
		{"éclair", "É"},
		{"123 go", "1"},
		{"", "?"},
	}
	for _, c := range cases {
		if got := firstLetter(c.name); got != c.want {
			t.Errorf("firstLetter(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDeriveColorDeterministic(t *testing.T) {
	a := deriveColor("Wishy Watchy")
	b := deriveColor("Wishy Watchy")
	if a != b {
		t.Errorf("deriveColor not deterministic: %v vs %v", a, b)
	}
}

func TestDeriveColorInRange(t *testing.T) {
	names := []string{"Wishy Watchy", "Hello World", "", "a", "Zzz Top", "menuet"}
	for _, n := range names {
		c := deriveColor(n)
		for _, v := range []float64{c.r, c.g, c.b} {
			if v < 0 || v > 1 {
				t.Errorf("deriveColor(%q) component %v out of [0,1]", n, v)
			}
		}
	}
}

func TestDeriveColorVaries(t *testing.T) {
	// Different names should generally yield different colors; guard against
	// a constant-color regression.
	if deriveColor("Alpha") == deriveColor("Beta") {
		t.Error("deriveColor produced identical colors for different names")
	}
}

func TestParseHexColor(t *testing.T) {
	c, err := parseHexColor("#4A90D9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := rgb{r: 74.0 / 255, g: 144.0 / 255, b: 217.0 / 255}
	if c != want {
		t.Errorf("parseHexColor = %v, want %v", c, want)
	}
	if _, err := parseHexColor("nope"); err == nil {
		t.Error("expected error for invalid color")
	}
}
