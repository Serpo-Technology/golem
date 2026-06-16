package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The media store is content-addressed: identical bytes hash to one name and
// are written once, and the reference resolves to the file on disk under
// <home>/media.
func TestMediaStoreContentAddressed(t *testing.T) {
	dir := t.TempDir()
	m, err := OpenMediaStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	pixels := []byte("\x89PNG fake image bytes")
	h1, err := m.Put(pixels)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Has(h1) {
		t.Fatal("a stored blob should be present")
	}
	h2, err := m.Put(pixels)
	if err != nil || h2 != h1 {
		t.Fatalf("identical media should dedupe to one hash: %q vs %q", h1, h2)
	}
	if h3, _ := m.Put([]byte("other bytes")); h3 == h1 {
		t.Fatal("different media should hash differently")
	}
	got, err := os.ReadFile(m.Path(h1))
	if err != nil || string(got) != string(pixels) {
		t.Fatalf("the stored blob should read back intact: %v", err)
	}
	if filepath.Dir(m.Path(h1)) != filepath.Join(dir, "media") {
		t.Fatalf("media should live under <home>/media: %s", m.Path(h1))
	}
	if m.Has("deadbeef") {
		t.Fatal("an absent hash should not report present")
	}
}

// A perceptual moment carries a media reference in a sparse side table; a
// moment captioned from nothing carries none.
func TestMediaReferenceOnFragment(t *testing.T) {
	s, _ := tempStore(t)
	seen := encodeAt(t, s, "vision", "a brown dog", 4, time.Now())
	plain := encodeAt(t, s, "tell", "a told sentence", 4, time.Now())
	if err := s.SetMedia(seen.ID, "abc123", "image/png"); err != nil {
		t.Fatal(err)
	}
	hash, mimeType, ok := s.Media(seen.ID)
	if !ok || hash != "abc123" || mimeType != "image/png" {
		t.Fatalf("the percept should carry its media reference: %q %q %v", hash, mimeType, ok)
	}
	if _, _, ok := s.Media(plain.ID); ok {
		t.Fatal("a moment with no media should report none")
	}
}
