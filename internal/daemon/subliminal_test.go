package daemon

import (
	"testing"
	"time"

	"golem/internal/senses"
)

// The graded gate lets learning continue past habituation. With a channel enabled for sub-conscious
// learning and its calibrator warmed so nothing clears consciousness, a repeated percept still ENCODES
// (below consciousness, at low salience) rather than being dropped, and enough of it earns a concept by
// accumulated mass -- all with habituation fully on. This is the always-online learner in miniature:
// no calibration_warmup flip, the percept forms because encoding was decoupled from consciousness.
func TestSubliminalEncodesAndEarnsConcept(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.SubliminalEnabled = true
	d.cfg.SubconsciousChannels = []string{"vision"}
	d.cfg.SurpriseThreshold = 0.0                           // keep even a fully-learned template above the noise floor
	d.cfg.EncodeBursts = map[string]float64{"vision": 1000} // this test is about genesis-by-mass, not the flood throttle
	d.calibrator("vision").Seed(200, 100.0, 1.0)            // warmed past warmup; nothing clears consciousness at mean 100

	base := time.Unix(1_000_000, 0)
	sub, con := 0, 0
	for i := 0; i < 20; i++ {
		r := d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
			At: base.Add(time.Duration(i) * time.Second)}, false, true)
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

	// Subliminal repetition earns the concept by accumulated mass, habituation on throughout.
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	if born, _ := d.growConcepts(); born == 0 {
		t.Fatal("enough subliminal showings should birth a concept via accumulated mass")
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

// A fast ambient flood on a sub-conscious channel is throttled by the sub-conscious governor, so a
// thousand identical frames arriving faster than the budget refills cannot storm the store nor mint a
// concept on the next nap by sheer volume. The governor runs on a budget separate from the conscious
// stream, so the flood also cannot starve a genuine conscious surprise of its tokens.
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
		r := d.process(senses.Observation{Channel: "vision", Content: "a small copper kettle",
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
		t.Fatalf("a throttled flood must not mint a concept in a single nap: born=%d", born)
	}
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
