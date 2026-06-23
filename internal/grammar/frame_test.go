package grammar

import (
	"reflect"
	"testing"
)

// Tokenize keeps a leading minus on a negative number so a negative literal reads
// as itself, while still trimming surrounding punctuation and leaving word-internal
// marks (and a non-numeric dash) alone.
func TestTokenizeKeepsNegativeNumbers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"the list of -5 and -2 and -8", []string{"the", "list", "of", "-5", "and", "-2", "and", "-8"}},
		{"(-8),", []string{"-8"}},
		{"self-build go.mod 3-5", []string{"self-build", "go.mod", "3-5"}},
		{"a dash - alone", []string{"a", "dash", "alone"}},
	}
	for _, c := range cases {
		if got, _ := Tokenize(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
