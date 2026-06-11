package config

import (
	"strings"
	"testing"
)

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
