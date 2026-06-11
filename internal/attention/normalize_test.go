package attention

import "testing"

func TestNormalizeCorpus(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"Ping seq=4731 latency=12ms", "Ping seq=<n> latency=<n>ms"},
		{"Ping seq=4732 latency=9ms", "Ping seq=<n> latency=<n>ms"},
		{"Connection refused peer=10.0.0.7:51334", "Connection refused peer=<ip>"},
		{"Disk full on /dev/disk1s5", "Disk full on <path>"},
		{"Segfault at 0x7ffee9c01234 in worker 3", "Segfault at <hex> in worker <n>"},
		{"job 8f14e45f-ceea-4671-9b2a-2a7e54f1f0a2 done", "job <uuid> done"},
		{"2026-06-09T14:03:22Z backup finished", "<ts> backup finished"},
		{"Jun  9 14:03:22 kernel wake", "<ts> kernel wake"},
		{"saved /Users/gokce/projects/golem/main.go", "saved <path>"},
		{"sha 3b1f9d2e4c5a6b7d8e9f0a1b2c3d4e5f", "sha <hex>"},
		{"a   b\tc", "a b c"},
		{"Ping", "Ping"},
	}
	for _, c := range cases {
		if got := Normalize(c.raw); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// Lines that differ only in their variable parts must share a template,
// because that identity is what lets the predictor treat them as one event.
func TestNormalizeStability(t *testing.T) {
	a := Normalize("Ping seq=1 latency=3ms")
	b := Normalize("Ping seq=99841 latency=140ms")
	if a != b {
		t.Fatalf("templates differ: %q vs %q", a, b)
	}
}
