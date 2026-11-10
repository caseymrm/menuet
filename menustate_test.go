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
