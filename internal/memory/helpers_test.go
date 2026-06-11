package memory

import (
	"path/filepath"
	"testing"
	"time"

	"golem/internal/attention"
)

// encodeAt stores one moment with a chosen surprise and time, the
// shorthand most store tests speak in.
func encodeAt(t *testing.T, s *Store, channel, content string, surprise float64, at time.Time) *Fragment {
	t.Helper()
	f := &Fragment{Channel: channel, Content: content, Template: attention.Normalize(content),
		Surprise: surprise, Time: at}
	if _, err := s.Encode(f); err != nil {
		t.Fatal(err)
	}
	return f
}

// edgeWeightRel reads one directed edge's weight, zero when absent.
func edgeWeightRel(t *testing.T, s *Store, from, to uint64, rel string) float64 {
	t.Helper()
	edges, err := s.LinksFromRel([]uint64{from}, rel)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.To == to {
			return e.Weight
		}
	}
	return 0
}

func edgeWeight(t *testing.T, s *Store, from, to uint64) float64 {
	t.Helper()
	return edgeWeightRel(t, s, from, to, RelTemporal)
}

// openTestStore opens a throwaway archive.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
