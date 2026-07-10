package daemon

import (
	"testing"
	"time"

	"golem/internal/senses"
)

// The graded gate lets learning continue past habituation. With a channel enabled for sub-conscious
// learning and its calibrator warmed so nothing clears consciousness, a percept that RECURS ACROSS
// SEPARATE EPISODES still ENCODES (below consciousness, at low salience) rather than being dropped, and
// enough distinct episodes earn a concept by accumulated mass -- all with habituation fully on. This is
// the always-online learner in miniature: no calibration_warmup flip, the percept forms because encoding
// was decoupled from consciousness. Showings are spaced past the episode window so each is a fresh
// episode; a flood within one episode would be de-duplicated (see TestSubliminalDedupsWithinEpisode).
func TestSubliminalEncodesAndEarnsConcept(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.SubliminalEnabled = true
	d.cfg.SubconsciousChannels = []string{"vision"}
	d.cfg.SurpriseThreshold = 0.0                           // keep even a fully-learned template above the noise floor
	d.cfg.EncodeBursts = map[string]float64{"vision": 1000} // this test is about genesis-by-mass, not the flood throttle
	d.calibrator("vision").Seed(200, 100.0, 1.0)            // warmed past warmup; nothing clears consciousness at mean 100

	base := time.Unix(1_000_000, 0)
	gap := d.cfg.EpisodeGap() + time.Second // each showing is a fresh episode
	sub, con := 0, 0
	for i := 0; i < 20; i++ {
		r := d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
			At: base.Add(time.Duration(i) * gap)}, false, true)
		if r.FragmentID == 0 {
			continue
		}
		if r.Conscious {
			con++
		} else {
			sub++
		}
	}
	if con != 0 {
		t.Fatalf("nothing should reach consciousness at this calibration: conscious=%d", con)
	}
	if sub == 0 {
		t.Fatal("a habituated percept on a sub-conscious channel should encode subliminally, not drop")
	}
	if got := d.stat("vision").Subliminal; got != sub {
		t.Fatalf("the subliminal encodes should be counted: stat=%d seen=%d", got, sub)
	}

	// Recurrence across distinct episodes earns the concept by accumulated mass, habituation on throughout.
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	if born, _ := d.growConcepts(); born == 0 {
		t.Fatal("enough separate subliminal episodes should birth a concept via accumulated mass")
	}
}

// Within one continuous exposure a habituated percept is de-duplicated: the first sighting encodes and
// every repeat inside the episode window is folded, not re-encoded, so staring at one thing cannot mint
// a concept by sheer volume. A thousand identical frames leave exactly one faint trace, far below the
// genesis threshold, and the folds are counted so the glass box shows what happened.
func TestSubliminalDedupsWithinEpisode(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.SubliminalEnabled = true
	d.cfg.SubconsciousChannels = []string{"vision"}
	d.cfg.SurpriseThreshold = 0.0
	d.cfg.EncodeBursts = map[string]float64{"vision": 100000} // isolate dedup from the flood throttle
	d.calibrator("vision").Seed(200, 100.0, 1.0)

	base := time.Unix(1_000_000, 0)
	step := d.cfg.EpisodeGap() / 10 // well inside the window: one continuous episode
	encoded := 0
	for i := 0; i < 1000; i++ {
		r := d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
			At: base.Add(time.Duration(i) * step)}, false, true)
		if r.FragmentID != 0 {
			encoded++
		}
	}
	if encoded != 1 {
		t.Fatalf("a continuous exposure should encode exactly once, not %d times", encoded)
	}
	if got := d.stat("vision").Deduped; got != 999 {
		t.Fatalf("the folded repeats should be counted: deduped=%d want 999", got)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	if born, _ := d.growConcepts(); born != 0 {
		t.Fatalf("one episode of a percept must not mint a concept: born=%d", born)
	}
}

// With sub-conscious learning off (the default), a habituated percept is dropped exactly as before, so
// no fragment encodes and no concept is born. This pins the flag-off path byte-identical to the old
// binary gate.
func TestSubliminalOffDropsHabituated(t *testing.T) {
	d, s := testDaemon(t)
	// SubliminalEnabled defaults to false.
	d.cfg.SurpriseThreshold = 0.0
	d.calibrator("vision").Seed(200, 100.0, 1.0)

	base := time.Unix(1_000_000, 0)
	for i := 0; i < 20; i++ {
		r := d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
			At: base.Add(time.Duration(i) * time.Second)}, false, true)
		if r.FragmentID != 0 {
			t.Fatalf("with sub-conscious learning off a habituated percept must drop, not encode")
		}
	}
	if got := d.stat("vision").Subliminal; got != 0 {
		t.Fatalf("no subliminal encodes should happen with the flag off: %d", got)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	if born, _ := d.growConcepts(); born != 0 {
		t.Fatalf("a dropped habituated percept should birth no concept: born=%d", born)
	}
}

// A fast flood of EVER-CHANGING percepts on a sub-conscious channel (which de-duplication cannot fold,
// since each is a fresh episode) is bounded by the sub-conscious governor: arriving faster than the
// budget refills, only the burst allowance encodes. The governor runs on a budget separate from the
// conscious stream, so the flood cannot starve a genuine conscious surprise of its tokens, and a churn
// of distinct one-off templates mints nothing.
func TestSubliminalFloodIsThrottled(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.SubliminalEnabled = true
	d.cfg.SubconsciousChannels = []string{"vision"}
	d.cfg.SurpriseThreshold = 0.0
	d.calibrator("vision").Seed(200, 100.0, 1.0)

	// The timestamps sit in one instant relative to the freshly-made bucket, so no budget refills:
	// only the burst allowance gets through, modelling a flood faster than the channel can refill.
	base := time.Unix(1_000_000, 0)
	encoded := 0
	for i := 0; i < 1000; i++ {
		r := d.process(senses.Observation{Channel: "vision", Content: "a copper kettle " + alphaWord(i),
			At: base.Add(time.Duration(i) * time.Millisecond)}, false, true)
		if r.FragmentID != 0 {
			encoded++
		}
	}
	if encoded > int(d.cfg.EncodeBurst)+1 {
		t.Fatalf("a subliminal flood must be throttled to the burst allowance: encoded=%d burst=%v", encoded, d.cfg.EncodeBurst)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	if born, _ := d.growConcepts(); born != 0 {
		t.Fatalf("a churn of distinct one-off templates must mint nothing: born=%d", born)
	}
}

// alphaWord maps an index to a distinct lowercase word ("a", "b", ... "z", "aa", ...), so a test can
// emit templates that Normalize keeps distinct (unlike numbers, which collapse to one shape).
func alphaWord(i int) string {
	s := ""
	for i++; i > 0; i /= 26 {
		i--
		s = string(rune('a'+i%26)) + s
	}
	return s
}

// A subliminal observation withholds salience: it never spikes noradrenaline, so a routine percept
// does not alarm the chemistry the way a conscious surprise does.
func TestSubliminalWithholdsSalience(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.SubliminalEnabled = true
	d.cfg.SubconsciousChannels = []string{"vision"}
	d.cfg.SurpriseThreshold = 0.0
	d.cfg.EncodeBursts = map[string]float64{"vision": 1000}
	d.calibrator("vision").Seed(200, 100.0, 1.0)

	before := d.chem.Snapshot().Noradrenaline
	for i := 0; i < 10; i++ {
		d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
			At: time.Unix(1_000_000, 0).Add(time.Duration(i) * time.Second)}, false, true)
	}
	if after := d.chem.Snapshot().Noradrenaline; after > before {
		t.Fatalf("a subliminal percept must not spike noradrenaline: %.4f -> %.4f", before, after)
	}
}
