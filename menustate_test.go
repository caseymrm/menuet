package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMenuStateColorImageSerializes(t *testing.T) {
	b, err := json.Marshal(&MenuState{Image: "shape", ColorImage: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"ColorImage":true`) {
		t.Errorf("ColorImage missing from JSON: %s", b)
	}
	b, err = json.Marshal(&MenuState{Image: "shape"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"ColorImage":false`) {
		t.Errorf("zero ColorImage should serialize as false: %s", b)
	}
}

func TestMenuStateBorderSerializes(t *testing.T) {
	b, err := json.Marshal(&MenuState{Title: "t", Border: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"Border":true`) {
		t.Errorf("Border missing from JSON: %s", b)
	}
	b, err = json.Marshal(&MenuState{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"Border":false`) {
		t.Errorf("zero Border should serialize as false: %s", b)
	}
}
