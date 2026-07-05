package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// traceOf fetches the trace of a claim by its canonical key, nil when none.
func traceOf(t *testing.T, d *Daemon, rel string, agent, patient uint64) *memory.Fragment {
	t.Helper()
	trace, err := d.store.FragmentByTemplate(claimTemplate(rel, agent, patient))
	if err != nil {
		t.Fatal(err)
	}
	if trace == nil || trace.Channel != claimChannel {
		return nil
	}
	return trace
}

// traceEdges partitions a trace's links by rel, so a test can read what the
// trace points at.
func traceEdges(t *testing.T, d *Daemon, traceID uint64) map[string][]uint64 {
	t.Helper()
	edges, err := d.store.LinksFrom([]uint64{traceID})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]uint64{}
	for _, e := range edges {
		out[e.Rel] = append(out[e.Rel], e.To)
	}
	return out
}

// An affirmed polar answer leaves its proof: the trace names the proposition
// through the said edges and points at the moment that holds it, and the probe
// reads the telling back to its teller.
func TestClaimTraceObserved(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	if got := say(t, d, "did the dog chase the cat").Say; got != "Yes, the dog chased the cat." {
		t.Fatalf("the observed proposition should affirm: %q", got)
	}
	trace := traceOf(t, d, "chase", dog.ID, cat.ID)
	if trace == nil {
		t.Fatal("an affirmed claim should leave a trace")
	}
	edges := traceEdges(t, d, trace.ID)
	if len(edges[memory.RelSaidAgent]) != 1 || edges[memory.RelSaidAgent][0] != dog.ID {
		t.Fatalf("the trace should name its agent: %v", edges[memory.RelSaidAgent])
	}
	if len(edges[memory.RelSaidPatient]) != 1 || edges[memory.RelSaidPatient][0] != cat.ID {
		t.Fatalf("the trace should name its patient: %v", edges[memory.RelSaidPatient])
	}
	if len(edges[memory.RelDrewOn]) != 1 {
		t.Fatalf("the observed claim should point at the one moment that holds it: %v", edges[memory.RelDrewOn])
	}
	a, p, rel, ok := d.claimAt(edges[memory.RelDrewOn][0])
	if !ok || a != dog.ID || p != cat.ID || rel != "chase" {
		t.Fatalf("the support should be the proposition's own moment: %d %d %q %v", a, p, rel, ok)
	}
	if got := say(t, d, "why do you say that").Say; got != "You told me." {
		t.Fatalf("an untagged telling is the architect's: %q", got)
	}
}

// A hedged guess stores the very basis it rode in on: the lender concept and
// the lender's own moment, so the probe speaks the likeness and the fact even
// after the resemblance layer would have drifted.
func TestClaimTraceInferredKeepsLender(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the guess should hedge: %q", got)
	}
	trace := traceOf(t, d, "chase", wolf.ID, cat.ID)
	if trace == nil {
		t.Fatal("a hedged claim should leave a trace")
	}
	edges := traceEdges(t, d, trace.ID)
	drew := map[uint64]bool{}
	for _, id := range edges[memory.RelDrewOn] {
		drew[id] = true
	}
	if !drew[dog.ID] {
		t.Fatalf("the trace should keep the lender the guess rode on: %v", edges[memory.RelDrewOn])
	}
	if len(edges[memory.RelDrewOn]) != 2 {
		t.Fatalf("the trace should keep the lender and its moment: %v", edges[memory.RelDrewOn])
	}
	got := say(t, d, "why do you say that").Say
	if !strings.Contains(got, "like a dog") {
		t.Fatalf("the probe should speak the likeness: %q", got)
	}
	if !strings.Contains(got, "The dog chased the cat.") {
		t.Fatalf("the probe should speak the fact the guess rested on: %q", got)
	}
	if !strings.Contains(got, "You told me.") {
		t.Fatalf("the probe should own where the fact came from: %q", got)
	}
}

// A conclusion contingent on a supposition never becomes a durable proof: the
// supposition dies with "never mind", and a trace would outlive it.
func TestSupposedClaimLeavesNoTrace(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	wolf := groundNoun(t, d, "wolf")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat") // the chase relation exists; the wolf's is only supposed
	say(t, d, "suppose the wolf chased the cat")
	got := say(t, d, "did the wolf chase the cat").Say
	if !strings.Contains(strings.ToLower(got), "if so") {
		t.Fatalf("a supposition-backed answer should be conditional: %q", got)
	}
	if traceOf(t, d, "chase", wolf.ID, cat.ID) != nil {
		t.Fatal("a supposition-backed claim must leave no trace")
	}
}

// A proposition heard from an enrolled peer is hedged, and its proof names the
// teller: the trace points at the told moment, whose source edge says who.
func TestJustifyNamesLowTrustTeller(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "bird")
	if got := sayAs(t, d, "llm", "a dog chased a bird").Say; got == "" {
		t.Fatal("a peer telling should land")
	}
	got := say(t, d, "did the dog chase the bird").Say
	if !strings.Contains(got, "robably") {
		t.Fatalf("a peer-told fact should answer hedged: %q", got)
	}
	why := say(t, d, "why do you say that").Say
	if !strings.Contains(why, "told me") || strings.Contains(why, "You told me") {
		t.Fatalf("the probe should name the teller, not the architect: %q", why)
	}
}

// A moment from a perceptual channel justifies as seen: the newest moment
// holding the proposition wins the lookup, and its channel says how it entered.
func TestJustifySeenEvidence(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	rel := d.resolveRelation("chase")
	if rel == nil {
		t.Fatal("the chase relation should resolve")
	}
	seen := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision", Content: "a dog chased a cat",
		Template: "seen dog chase cat", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(seen); err != nil {
		t.Fatal(err)
	}
	if err := s.Propose(rel.ID, seen.ID, dog.ID, cat.ID, 0.9); err != nil {
		t.Fatal(err)
	}
	if got := say(t, d, "did the dog chase the cat").Say; got != "Yes, the dog chased the cat." {
		t.Fatalf("the observed proposition should affirm: %q", got)
	}
	if got := say(t, d, "why do you say that").Say; got != "I saw it." {
		t.Fatalf("a perceptual moment justifies as seen: %q", got)
	}
}

// The probe speaks the asked tongue: a Turkish ask reads the same trace and
// answers with the Turkish provenance surface.
func TestJustifyTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat")
	if got := sayLang(t, d, "tr", "neden öyle diyorsun").Say; got != "Sen söyledin." {
		t.Fatalf("the Turkish probe should answer in Turkish: %q", got)
	}
}

// With nothing asserted there is nothing to justify, and a turn that asserts
// nothing traceable drops the last proof rather than leaving it to be
// misattributed to a fresh reply.
func TestJustifyNothingToPointTo(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	if got := say(t, d, "why do you say that").Say; got != "I cannot say why." {
		t.Fatalf("with no claim there is no proof to read: %q", got)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat")
	say(t, d, "hello")
	if got := say(t, d, "why do you say that").Say; got != "I cannot say why." {
		t.Fatalf("a fresh reply must not wear a stale proof: %q", got)
	}
}

// The probe can be asked twice: reading the proof is not asserting, so the
// trace stays in mind for a second ask.
func TestJustifyRepeats(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat")
	if got := say(t, d, "why do you say that").Say; got != "You told me." {
		t.Fatalf("the first ask should read the proof: %q", got)
	}
	if got := say(t, d, "how do you know").Say; got != "You told me." {
		t.Fatalf("the second ask should read it again: %q", got)
	}
}

// A why answer asserts the cause clause, and its proof is the telling that
// laid the cause: the probe owns the teller without re-telling the clause it
// just spoke.
func TestJustifyWhyCause(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "bone")
	say(t, d, "a dog chased a cat because the cat grabbed the bone")
	got := say(t, d, "why did the dog chase the cat").Say
	if !strings.Contains(got, "Because the cat grabbed the bone.") {
		t.Fatalf("the told cause should answer the why: %q", got)
	}
	if got := say(t, d, "why do you say that").Say; got != "You told me." {
		t.Fatalf("the cause claim's proof is its telling: %q", got)
	}
}

// A told membership's trace is a stub the probe re-reads live: the telling is
// the membership machinery's own record.
func TestJustifyToldIsA(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "daemon")
	say(t, d, "you are a daemon")
	got := say(t, d, "are you a daemon").Say
	if !strings.Contains(got, "Probably") {
		t.Fatalf("a told membership should hedge: %q", got)
	}
	if got := say(t, d, "why do you say that").Say; got != "I was told so." {
		t.Fatalf("a told membership justifies by the telling: %q", got)
	}
}

// A repair cuts the claims that drew on the retracted fact, not only the
// claims that named it: the guess that leaned on the overturned moment
// retracts with it, and its proof goes too.
func TestRepairCutsDependentClaims(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	groundNoun(t, d, "rabbit")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)
	say(t, d, "did the dog chase the cat")
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the guess should stand before the repair: %q", got)
	}
	say(t, d, "no, the dog chased the rabbit")
	if traceOf(t, d, "chase", dog.ID, cat.ID) != nil {
		t.Fatal("the retracted claim's own trace should go with it")
	}
	if traceOf(t, d, "chase", wolf.ID, cat.ID) != nil {
		t.Fatal("the dependent claim's trace should be cut at its support")
	}
	if got := say(t, d, "did the wolf chase the cat").Say; got != "No, the wolf did not chase the cat." {
		t.Fatalf("a guess whose support was corrected away should no longer hold: %q", got)
	}
}

// A trace is invisible to the proposition machinery: the trace of a guess must
// never upgrade the guess to an observed fact, and it holds no role edges for
// the fact walks to find.
func TestTraceInvisibleToPropositionMoment(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)
	say(t, d, "did the wolf chase the cat")
	rel := d.resolveRelation("chase")
	if rel == nil {
		t.Fatal("the chase relation should resolve")
	}
	if _, ok := d.propositionMoment(rel, wolf.ID, cat.ID); ok {
		t.Fatal("a trace must never read as the proposition's moment")
	}
	got := strings.ToLower(say(t, d, "did the wolf chase the cat").Say)
	if !strings.Contains(got, "probably") || strings.Contains(got, "yes,") {
		t.Fatalf("the guess must stay a guess however often it is traced: %q", got)
	}
}

// A trace holds nothing alive: a moment kept only by a drewon edge still dies
// at the prune, a concept named only by said edges dies of disconnection from
// the grounded graph, and the trace itself decays like any episodic moment.
func TestTraceDoesNotSpareSupports(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	moment := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "tell", Content: "a fox jumped",
		Template: "fox jump", Weight: 0.06, Limbic: d.chem.Snapshot(), Time: now}
	if _, err := s.Encode(moment); err != nil {
		t.Fatal(err)
	}
	orphan := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "phantom",
		Template: "phantom", Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := s.Encode(orphan); err != nil {
		t.Fatal(err)
	}
	trace := &memory.Fragment{Kind: memory.KindEpisodic, Channel: claimChannel, Content: "claim",
		Template: claimTemplate("jump", orphan.ID, moment.ID), Weight: 0.5, Limbic: d.chem.Snapshot(), Time: now}
	if _, err := s.Encode(trace); err != nil {
		t.Fatal(err)
	}
	s.LinkRel(trace.ID, moment.ID, memory.RelDrewOn, 0.5)
	s.LinkRel(trace.ID, orphan.ID, memory.RelSaidAgent, 0.5)
	if _, err := s.Sleep(0.2, 0.05); err != nil {
		t.Fatal(err)
	}
	if frags, _ := s.FragmentsByIDs([]uint64{moment.ID}); len(frags) != 0 {
		t.Fatal("a moment held only by a trace should still die at the prune")
	}
	if frags, _ := s.FragmentsByIDs([]uint64{orphan.ID}); len(frags) != 0 {
		t.Fatal("a concept held only by said edges should still die of disconnection")
	}
	// The trace itself is episodic: a night's decay thins it toward the floor.
	if frags, _ := s.FragmentsByIDs([]uint64{trace.ID}); len(frags) == 1 {
		if frags[0].Weight >= 0.5 {
			t.Fatalf("a trace should decay like any moment: %v", frags[0].Weight)
		}
	}
}

// When the proof has decayed out from under a claim, the probe admits the
// loss rather than inventing a basis.
func TestJustifyAfterEvidenceDecay(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat")
	trace := traceOf(t, d, "chase", dog.ID, cat.ID)
	if trace == nil {
		t.Fatal("the claim should have left a trace")
	}
	for _, id := range traceEdges(t, d, trace.ID)[memory.RelDrewOn] {
		if _, err := d.store.DeleteByID(id); err != nil {
			t.Fatal(err)
		}
	}
	// The discourse thread would still vouch for the fact this exchange; the
	// decay this simulates happens across nights, after the thread has cleared.
	d.threadProps = nil
	if got := say(t, d, "why do you say that").Say; got != "I no longer remember why." {
		t.Fatalf("a proof that has faded is admitted, not invented: %q", got)
	}
}

// Re-asserting a claim refreshes its one trace instead of minting a twin, so
// the record stays one entry per proposition.
func TestTraceRefreshesInPlace(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat")
	say(t, d, "did the dog chase the cat")
	frags, err := d.store.EpisodesByExactShape(claimTemplate("chase", dog.ID, cat.ID), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 {
		t.Fatalf("one claim should keep one trace, got %d", len(frags))
	}
}
