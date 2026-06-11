package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// config.example.json at the repository root is the defaults: the same
// content Load writes to a fresh home on first run. The comparison is
// structural (values and key set, not formatting), so a default changed
// in code or a field added without updating the example turns this red.
func TestExampleConfigMatchesDefaults(t *testing.T) {
	example, err := os.ReadFile("../../config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	var fromExample Config
	if err := json.Unmarshal(example, &fromExample); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fromExample, Default()) {
		t.Fatalf("config.example.json values have drifted from Default():\nexample %+v\ndefault %+v",
			fromExample, Default())
	}

	// The key sets must match too, or a renamed field would linger in
	// the example as silently ignored noise.
	var exampleKeys map[string]any
	if err := json.Unmarshal(example, &exampleKeys); err != nil {
		t.Fatal(err)
	}
	defaults, err := json.Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	var defaultKeys map[string]any
	if err := json.Unmarshal(defaults, &defaultKeys); err != nil {
		t.Fatal(err)
	}
	for k := range defaultKeys {
		if _, ok := exampleKeys[k]; !ok {
			t.Errorf("config.example.json is missing %q", k)
		}
	}
	for k := range exampleKeys {
		if _, ok := defaultKeys[k]; !ok {
			t.Errorf("config.example.json carries unknown field %q", k)
		}
	}
}

// Every denylist entry must actually be caught, so dropping any one of
// them turns this test red. Matching is case-insensitive.
func TestDenylist(t *testing.T) {
	for _, entry := range denylist {
		cmd := "please " + entry + " now"
		if !Denylisted(cmd) {
			t.Errorf("denylist entry %q is not enforced", entry)
		}
		if !Denylisted(strings.ToUpper(cmd)) {
			t.Errorf("denylist entry %q is not matched case-insensitively", entry)
		}
	}
	for _, ok := range []string{"go test ./...", "git status", "make build", "git push origin main"} {
		if Denylisted(ok) {
			t.Errorf("%q should not be denylisted", ok)
		}
	}
}
