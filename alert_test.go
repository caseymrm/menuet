package menuet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputTypeConstants(t *testing.T) {
	// The Objective-C side reads the Type field as an integer and compares
	// against 1 for the password case (see alert.m). Lock the iota values
	// in so future reordering doesn't silently swap the input rendering.
	if InputText != 0 {
		t.Errorf("InputText = %d, want 0", InputText)
	}
	if InputPassword != 1 {
		t.Errorf("InputPassword = %d, want 1; alert.m checks `type == 1` for masked fields", InputPassword)
	}
}

func TestAlertJSONIncludesInputType(t *testing.T) {
	// The Alert value crosses the cgo boundary as JSON. Make sure the Type
	// field is present on every input so a password input doesn't quietly
	// degrade to a plain text field.
	a := Alert{
		MessageText: "Login",
		Buttons:     []string{"OK", "Cancel"},
		Inputs: []AlertInput{
			{Placeholder: "Username"},
			{Placeholder: "Password", Type: InputPassword},
		},
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(b)

	// Both inputs should serialize their Type field, with the password
	// input emitting Type:1.
	wantSubstrings := []string{
		`"Placeholder":"Username"`,
		`"Type":0`,
		`"Placeholder":"Password"`,
		`"Type":1`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Errorf("Alert JSON missing %q\nfull JSON: %s", want, got)
		}
	}
}

func TestAlertJSONRoundTripPreservesTypedInputs(t *testing.T) {
	original := Alert{
		MessageText: "Login",
		Inputs: []AlertInput{
			{Placeholder: "Username", Type: InputText},
			{Placeholder: "Password", Type: InputPassword},
		},
	}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var round Alert
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(round.Inputs) != 2 {
		t.Fatalf("round-tripped %d inputs, want 2", len(round.Inputs))
	}
	if round.Inputs[0].Type != InputText {
		t.Errorf("Inputs[0].Type = %d, want InputText (0)", round.Inputs[0].Type)
	}
	if round.Inputs[1].Type != InputPassword {
		t.Errorf("Inputs[1].Type = %d, want InputPassword (1)", round.Inputs[1].Type)
	}
}
