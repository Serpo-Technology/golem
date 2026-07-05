package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
	"golem/wire"
)

// sayAs speaks as a named peer, the way a source other than the local operator
// reaches the channel. The words are that peer's, heard at its trust.
func sayAs(t *testing.T, d *Daemon, source, words string) wire.Response {
	t.Helper()
	resp, _, _ := d.handle(wire.Request{Cmd: "tell", Content: words, Source: source}, nil)
	if !resp.OK {
		t.Fatalf("tell %q as %q failed: %s", words, source, resp.Error)
	}
	return resp
}

// agentWeight is the weight of the agent edge a telling laid down, the strength of the
// proposition it grounded.
func agentWeight(t *testing.T, d *Daemon, s *memory.Store, r wire.Response, agentPhrase string) float64 {
	t.Helper()
	c := d.resolveConcept(agentPhrase)
	if c == nil || r.Observe == nil {
		t.Fatalf("the telling should ground %q and store", agentPhrase)
	}
	return edgeWeightRel(t, s, r.Observe.FragmentID, c.ID, memory.RelAgent)
}

// A peer's telling is tagged with its source and written weaker than the operator's, so
// a low-trust claim enters defeasibly. (Both edges also gain the same retrieval-practice
// bump from the post-telling spread, so trust shows in the gap, not an exact weight.)
func TestPeerTellIsSourceTaggedAndDefeasible(t *testing.T) {
	d, s := testDaemon(t)

	// Same clause shape for both, so the only difference is the source's trust.
	rA := say(t, d, "a wolf chased a rabbit")
	wA := agentWeight(t, d, s, rA, "wolf")
	if edges, _ := s.LinksFromRel([]uint64{rA.Observe.FragmentID}, memory.RelSource); len(edges) != 0 {
		t.Fatal("an untagged operator telling must carry no source tag")
	}

	rL := sayAs(t, d, "llm", "a bear chased a fish")
	wL := agentWeight(t, d, s, rL, "bear")
	if wL >= wA {
		t.Fatalf("a low-trust peer telling (%.2f) must write weaker than the operator's (%.2f)", wL, wA)
	}
	// The gap is the trust difference in the proposition weight (the shared practice
	// bump cancels), so a default-trust peer sits well below the operator.
	if gap := wA - wL; gap < comprehendWeight*(1-defaultSourceTrust)-1e-6 {
		t.Fatalf("the peer's telling should be weaker by the trust gap ~%.2f, got %.2f", comprehendWeight*(1-defaultSourceTrust), gap)
	}
	edges, _ := s.LinksFromRel([]uint64{rL.Observe.FragmentID}, memory.RelSource)
	if len(edges) != 1 {
		t.Fatalf("a peer telling must be tagged with one source, got %d", len(edges))
	}
	src, _ := s.FragmentsByIDs([]uint64{edges[0].To})
	if len(src) != 1 || src[0].Category != sourceCategory || src[0].Content != "llm" {
		t.Fatalf("the source tag should point at the llm source node, got %+v", src)
	}
}

// A grounded denial disposes: a low-trust source that asserts what GOLEM was told does
// not hold is doubted, and the positive is not written.
func TestGroundedDenialDoubtsPeerClaim(t *testing.T) {
	d, _ := testDaemon(t)
	// The operator grounds the concepts, then a denial.
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog does not chase a bird")
	dog, bird := d.resolveConcept("dog"), d.resolveConcept("bird")
	if dog == nil || bird == nil {
		t.Fatal("dog and bird should be grounded")
	}
	if !d.propositionDenied(dog, "chase", bird.ID) {
		t.Fatal("the operator's denial should stand")
	}
	// A peer asserts the denied positive: it is doubted and voiced.
	rL := sayAs(t, d, "llm", "a dog chased a bird")
	if rL.Say == "" {
		t.Fatal("a doubted claim should be voiced, not received in silence")
	}
	// The claim is on record now (for the source's track record) and held as a doubt,
	// but the grounded denial keeps standing over the read: the peer does not overturn it.
	if b, _, _, _ := d.assessProposition(dog, "chase", bird); b != beliefDenied {
		t.Fatalf("the grounded denial should still dominate a doubted claim, got belief %d", b)
	}
	if p := d.held.findID("chase", dog.ID, bird.ID, heldDoubted); p == nil {
		t.Fatal("the doubted claim should be held")
	}
}

// Trust is monotonic in how firmly a telling is written: a default peer weakest, a
// vouched source firmer, the operator (an untagged local telling) firmest. Vouching
// for a source raises how firmly its claims are written.
func TestDelegatedTrust(t *testing.T) {
	d, s := testDaemon(t)

	// Same clause shape throughout, so the practice bump is uniform and only trust varies.
	wOperator := agentWeight(t, d, s, say(t, d, "a wolf chased a rabbit"), "wolf")
	wDefault := agentWeight(t, d, s, sayAs(t, d, "llm", "a bear chased a fish"), "bear")

	if err := d.setSourceTrust("llm", 0.9); err != nil {
		t.Fatal(err)
	}
	wVouched := agentWeight(t, d, s, sayAs(t, d, "llm", "a cat chased a mouse"), "cat")

	if !(wDefault < wVouched && wVouched < wOperator) {
		t.Fatalf("trust should raise write strength monotonically: default %.2f, vouched %.2f, operator %.2f", wDefault, wVouched, wOperator)
	}
}

// A declared source is always heard as that peer, never as the architect: the
// architect speaks untagged, so a telling that declares the name "architect"
// resolves to a low-seeded peer node, not to full trust. The signature that
// authenticates a network transducer authorizes injection; it never turns a
// declared peer's words into the architect's.
func TestArchitectNameCannotBeSpoofed(t *testing.T) {
	d, _ := testDaemon(t)
	// Even with a valid architect signature on the connection, a declared source
	// is the peer: the transducer relays the peer's words at the peer's trust.
	d.reqPrincipal = principal{architectSigned: true}
	name, trust := d.resolveSpeaker(wire.Request{Source: architectSource})
	if trust >= architectTrust {
		t.Fatalf("a declared claim to be the architect must not earn full trust, got %.2f", trust)
	}
	if name != architectSource {
		t.Fatalf("the declared name is kept, heard low: %q", name)
	}
}

// Trust moves through conversation, not a command: "trust the llm" raises it, "doubt the
// llm" lowers it, and only a trusted speaker may do so, so a peer cannot vouch for itself.
func TestConversationalTrustMove(t *testing.T) {
	d, _ := testDaemon(t)
	sayAs(t, d, "llm", "a bear chased a fish") // introduce the llm source (low trust)
	llm, err := d.ensureSource("llm")
	if err != nil {
		t.Fatal(err)
	}
	before := d.sourceTrust(llm.ID)

	r := say(t, d, "trust the llm")
	raised := d.sourceTrust(llm.ID)
	if raised <= before {
		t.Fatalf("talking should raise trust: before %.2f, after %.2f (said %q)", before, raised, r.Say)
	}

	say(t, d, "doubt the llm")
	if lowered := d.sourceTrust(llm.ID); lowered >= raised {
		t.Fatalf("doubting should lower trust: %.2f then %.2f", raised, lowered)
	}

	// A low-trust peer cannot move trust by talking, not even its own.
	held := d.sourceTrust(llm.ID)
	sayAs(t, d, "llm", "trust the llm")
	if now := d.sourceTrust(llm.ID); now != held {
		t.Fatalf("a peer must not move trust by talking: %.2f then %.2f", held, now)
	}
}

// A doubt is held as memory, not a passing remark: a low-trust claim contradicting a
// grounded denial leaves a held doubt GOLEM keeps.
func TestDoubtHeldAsMemory(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog does not chase a bird")
	dog, bird := d.resolveConcept("dog"), d.resolveConcept("bird")
	if dog == nil || bird == nil {
		t.Fatal("dog and bird should be grounded")
	}
	sayAs(t, d, "llm", "a dog chased a bird") // doubted, not written
	if p := d.held.findID("chase", dog.ID, bird.ID, heldDoubted); p == nil {
		t.Fatal("the doubt should be held as memory, not only voiced")
	}
}

// A source loses trust when its claims turn out to be contradicted by what GOLEM has
// grounded, the earned half of the model.
func TestTrustLearningLowersOnContradiction(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "a dog does not chase a bird") // grounded denials
	say(t, d, "a cat does not chase a fish")
	sayAs(t, d, "llm", "a dog chased a bird") // the peer claims the denied things
	sayAs(t, d, "llm", "a cat chased a fish")
	llm, _ := d.ensureSource("llm")
	if err := d.setSourceTrust("llm", 0.6); err != nil { // start mid so a drop is visible
		t.Fatal(err)
	}
	before := d.sourceTrust(llm.ID)
	d.growSourceTrust()
	if after := d.sourceTrust(llm.ID); after >= before {
		t.Fatalf("a source GOLEM has grounded otherwise should lose trust: %.2f then %.2f", before, after)
	}
}

// A source whose claims GOLEM has had no reason to deny recovers a little trust: earned
// slowly, the gentle positive while a grounded confirmation (perception) is still absent.
func TestTrustLearningRisesOnCleanRecord(t *testing.T) {
	d, _ := testDaemon(t)
	sayAs(t, d, "llm", "a zebra chased a lion")
	sayAs(t, d, "llm", "a moose chased a wolf")
	llm, _ := d.ensureSource("llm")
	if err := d.setSourceTrust("llm", 0.3); err != nil {
		t.Fatal(err)
	}
	before := d.sourceTrust(llm.ID)
	d.growSourceTrust()
	if after := d.sourceTrust(llm.ID); after <= before {
		t.Fatalf("an unchallenged record should restore a little trust: %.2f then %.2f", before, after)
	}
}

// The record earns trust but does not undo a grant: a clean night never pulls a source
// the architect vouched higher back down to the earned ceiling.
func TestTrustLearningDoesNotUndoAGrant(t *testing.T) {
	d, _ := testDaemon(t)
	sayAs(t, d, "llm", "a zebra chased a lion")
	if err := d.setSourceTrust("llm", 1.0); err != nil { // architect grants full trust
		t.Fatal(err)
	}
	llm, _ := d.ensureSource("llm")
	d.growSourceTrust()
	if got := d.sourceTrust(llm.ID); got < 1.0 {
		t.Fatalf("a clean night must not pull a granted source below its grant: got %.2f", got)
	}
}

// A fact heard only from a low-trust peer answers hedged, in the polar and the wh
// alike: the claim is on record at the teller's trust, so GOLEM reports it as
// probable rather than witnessed. The architect telling the same fact firms it.
func TestPeerToldFactAnswersHedged(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a striped gray zebra", 5, base)
	feedShape(t, s, "vision", "a green grass tuft", 5, base)
	feedShape(t, s, "vision", "a large brown horse", 5, base)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	// The relation itself is architect-taught vocabulary (on another agent), so the
	// contested FACT is the peer's alone and its patient minted fresh rather than
	// twin-bound onto the architect's thread.
	say(t, d, "a horse ate a grass")
	sayAs(t, d, "llm", "a zebra ate a mouse")

	polar := say(t, d, "did a zebra eat a mouse").Say
	if !strings.Contains(strings.ToLower(polar), "probab") {
		t.Fatalf("a peer-told fact should answer hedged, got %q", polar)
	}
	firm := say(t, d, "did a horse eat a grass").Say
	if !strings.Contains(strings.ToLower(firm), "yes") {
		t.Fatalf("an architect-told fact stays firm, got %q", firm)
	}
	// The architect confirming the peer's claim firms it.
	say(t, d, "a zebra ate a mouse")
	upgraded := say(t, d, "did a zebra eat a mouse").Say
	if !strings.Contains(strings.ToLower(upgraded), "yes") {
		t.Fatalf("a fully trusted telling should firm the fact, got %q", upgraded)
	}
}
