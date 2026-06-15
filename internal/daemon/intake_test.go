package daemon

import (
	"testing"
	"time"

	"golem/internal/senses"
)

// Reading a line on an untrusted channel mints its content words into the
// lexicon, so the vocabulary grows from what GOLEM reads and not only from what
// it is taught.
func TestReadingGrowsVocabulary(t *testing.T) {
	d, _ := testDaemon(t)
	line := "encyclopedia knowledge compendium reference"
	res := d.process(senses.Observation{Channel: "web", Content: line, At: time.Now()}, false, true)

	d.readVocabulary(line, res.FragmentID)

	if _, minted, err := d.store.WordNode("encyclopedia", "en", d.chem.Snapshot(), time.Now()); err != nil {
		t.Fatal(err)
	} else if minted {
		t.Fatal("reading the line should have minted its content words into the lexicon")
	}
}

// A bulk channel takes its per-channel encode burst while a trickle channel
// keeps the default, so the web can bank several sources in one cycle instead of
// starving every page after the first.
func TestPerChannelEncodeBurst(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.EncodeBurst = 6
	d.cfg.EncodeBursts = map[string]float64{"web": 50}

	if got := d.governor("web").max; got != 50 {
		t.Fatalf("the web governor should take its per-channel burst: %v", got)
	}
	if got := d.governor("syslog").max; got != 6 {
		t.Fatalf("an unlisted channel should keep the default burst: %v", got)
	}
}
