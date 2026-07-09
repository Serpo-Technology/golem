package daemon

import (
	"fmt"
	"reflect"
	"testing"

	"golem/internal/memory"
)

// teachNations grounds a few nation frames (each nation uses a currency, rules a capital, speaks a
// language, all distinct) and encodes their holographic records, the parallel-frame structure the
// structural analogy maps between. It returns the grounded concepts by gloss.
func teachNations(t *testing.T, d *Daemon) map[string]*memory.Fragment {
	t.Helper()
	c := map[string]*memory.Fragment{}
	for _, n := range []string{
		"america", "mexico", "france",
		"dollar", "peso", "euro",
		"washington", "mexicocity", "paris",
		"english", "spanish", "french",
	} {
		c[n] = groundNoun(t, d, n)
	}
	say(t, d, "the america minted the dollar")
	say(t, d, "the america ruled the washington")
	say(t, d, "the america spoke the english")
	say(t, d, "the mexico minted the peso")
	say(t, d, "the mexico ruled the mexicocity")
	say(t, d, "the mexico spoke the spanish")
	say(t, d, "the france minted the euro")
	say(t, d, "the france ruled the paris")
	say(t, d, "the france spoke the french")
	d.growHolographic()
	return c
}

// The structural analogy the other two modes cannot reach: the dollar of America answered by the peso
// of Mexico. No told relation connects the dollar to America (America USES the dollar, so the dollar
// is only ever a patient), and the featureless nation/currency nouns carry no kind vector for an
// offset, so both the symbolic and the offset modes gap. The holographic map holds each nation's whole
// frame and carries the currency role from one onto the other, proposed as a thin "perhaps".
func TestHoloAnalogyMapsFrames(t *testing.T) {
	d, _ := testDaemon(t)
	c := teachNations(t, d)

	// The offset mode cannot ride featureless nouns, so it is not what answers here.
	if _, ok := d.vectorAnalogy(c["dollar"], c["america"], c["mexico"]); ok {
		t.Fatal("featureless nouns have no kind vector; the offset mode should gap, leaving the holographic map")
	}
	got, ok := d.holoAnalogy(c["dollar"], c["america"], c["mexico"])
	if !ok || got != c["peso"].ID {
		t.Fatalf("the dollar of America should map to the peso of Mexico: id=%d ok=%v (peso=%d)", got, ok, c["peso"].ID)
	}

	// End to end through the proportion parse and the three-mode fallback: the holographic guess spoken.
	if line := say(t, d, "the dollar is to america as mexico is to what").Say; line != "Perhaps the peso." {
		t.Fatalf("the spoken proportion should offer the peso as a perhaps: %q", line)
	}
	// The mapping generalizes to another role and another nation: the capital of France.
	if got, ok := d.holoAnalogy(c["washington"], c["america"], c["france"]); !ok || got != c["paris"].ID {
		t.Fatalf("the capital of France should map to Paris: id=%d ok=%v (paris=%d)", got, ok, c["paris"].ID)
	}
}

// The holographic map admits the gap honestly. A concept with too thin a frame has no record, so a
// proportion over it has nothing to map between; and a filler drawn from the wrong frame maps to
// noise the two-part gate rejects rather than a forced fourth term.
func TestHoloAnalogyAdmitsGap(t *testing.T) {
	d, _ := testDaemon(t)
	c := teachNations(t, d)

	// A lone concept never taught a frame has no record, so a proportion touching it gaps.
	rock := groundNoun(t, d, "rock")
	if _, ok := d.holoAnalogy(c["dollar"], c["america"], rock); ok {
		t.Fatal("a target with no holographic record should admit the gap, not force a fourth term")
	}
	if _, ok := d.holoAnalogy(c["dollar"], rock, c["mexico"]); ok {
		t.Fatal("a source with no holographic record should admit the gap")
	}
	// The peso is Mexico's filler, not America's: binding it into Mexico's frame and unbinding by
	// America's yields noise the two-part gate rejects, not a forced answer.
	if id, ok := d.holoAnalogy(c["peso"], c["america"], c["mexico"]); ok {
		t.Fatalf("a filler foreign to the source frame should gap, got id=%d", id)
	}
}

// A holographic answer stays a hedged, graph-read-only proposal: it is spoken at the weakest hedge,
// it is re-derived identically each turn (never promoted to a firm fact), and it writes no belief, so
// the guess never becomes something GOLEM later asserts.
func TestHoloAnalogyStaysHedgedAndReadOnly(t *testing.T) {
	d, s := testDaemon(t)
	teachNations(t, d)

	first := say(t, d, "the dollar is to america as mexico is to what").Say
	if first != "Perhaps the peso." {
		t.Fatalf("the holographic guess should be hedged: %q", first)
	}
	// Re-asking yields the same hedged guess: it was never hardened into a firm answer.
	if again := say(t, d, "the dollar is to america as mexico is to what").Say; again != first {
		t.Fatalf("the guess should be stateless, not promoted: %q then %q", first, again)
	}
	// Nothing was written to the belief store by the geometric guess.
	if bs, err := s.Beliefs(); err != nil {
		t.Fatal(err)
	} else if len(bs) != 0 {
		t.Fatalf("a holographic guess should write no belief, got %d", len(bs))
	}
}

// The gate is stated in the codebook's own units, so the map still fires against a large brain where a
// fixed cosine margin would erode: the correct fourth term stands out as an outlier above the rising
// runner-up noise, and a frame-foreign filler still gaps.
func TestHoloAnalogyHoldsAtScale(t *testing.T) {
	d, _ := testDaemon(t)
	c := teachNations(t, d)
	// Flood the codebook so the runner-up order statistic rises the way it does on a brain of a
	// thousand concepts; the outlier gate must survive it.
	for i := 0; i < 800; i++ {
		groundNoun(t, d, fmt.Sprintf("filler%d", i))
	}
	if got, ok := d.holoAnalogy(c["dollar"], c["america"], c["mexico"]); !ok || got != c["peso"].ID {
		t.Fatalf("the map should still find the peso against a large codebook: id=%d ok=%v", got, ok)
	}
	if id, ok := d.holoAnalogy(c["peso"], c["america"], c["mexico"]); ok {
		t.Fatalf("a frame-foreign filler should gap even against a large codebook, got %d", id)
	}
}

// The nightly encode is a pure function of the settled graph, not the discourse thread: a lingering
// conversation that mentioned only one filler of a multi-filler role must not suppress the sibling
// filler from the record. Without the graph-only encode, gatherFacts would mark the whole role covered
// and drop the unspoken filler, making the record depend on the last exchange.
func TestHoloRecordIgnoresDiscourseThread(t *testing.T) {
	d, s := testDaemon(t)
	am := groundNoun(t, d, "america")
	dollar := groundNoun(t, d, "dollar")
	groundNoun(t, d, "eagle")
	groundNoun(t, d, "washington")
	say(t, d, "the america minted the dollar")
	say(t, d, "the america minted the eagle") // a second filler of the minted role
	say(t, d, "the america ruled the washington")

	var mintedRel string
	for _, f := range d.gatherFacts(am, "", "") {
		if f.coID == dollar.ID {
			mintedRel = f.rel
		}
	}
	if mintedRel == "" {
		t.Fatal("setup: america should mint the dollar")
	}

	d.threadProps = nil
	d.growHolographic()
	clean, ok := s.Holo(am.ID)
	if !ok {
		t.Fatal("america should have a record")
	}

	// A lingering thread mentioning only the dollar under the minted role must not thin the record.
	d.threadProps = []threadProp{{rel: mintedRel, agent: am.ID, patient: dollar.ID}}
	d.growHolographic()
	withThread, _ := s.Holo(am.ID)

	if !reflect.DeepEqual(clean, withThread) {
		t.Fatal("the holographic record must be a pure function of the graph, not the discourse thread")
	}
}
