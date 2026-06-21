package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// A synonym imperative reaches the same faculty the exact verb does, the
// imperativeIntents hardcoding dissolved into the concept graph.
func TestBuiltinCommandSynonymRoutes(t *testing.T) {
	exact := func(words string) string {
		d, _ := testDaemon(t)
		groundNoun(t, d, "rabbit")
		return say(t, d, words).Say
	}
	if got, want := exact("recollect the rabbit"), exact("recall the rabbit"); got != want {
		t.Errorf("recollect should route to recall: %q, want %q", got, want)
	}
	if got, want := exact("reflect"), exact("introspect"); got != want {
		t.Errorf("reflect should route to introspect: %q, want %q", got, want)
	}
}

// A faculty named in another tongue resolves to the same command concept, so the
// cross-tongue layer carries the dissolved commands for free.
func TestBuiltinCommandCrossLanguage(t *testing.T) {
	d, _ := testDaemon(t)
	c := d.resolveConcept("hatırla")
	if c == nil || d.commandName[c.ID] != "recall" {
		t.Fatalf("the Turkish recall verb should name the recall faculty, got %v", c)
	}
	if got := sayLang(t, d, "tr", "hatırla").Say; got == "" {
		t.Errorf("a Turkish recall imperative should run the recall faculty, got empty")
	}
}

// A faculty that needs an argument asks for it when the imperative gives none,
// the arity check the closed command set affords.
func TestBuiltinCommandArity(t *testing.T) {
	d, _ := testDaemon(t)
	if got := say(t, d, "remember").Say; !strings.Contains(got, "recall") {
		t.Errorf("recall with no topic should ask what to recall: %q", got)
	}
}

// A verb naming no faculty but resembling what one concerns routes there. The
// grammar gates this on the verb being an imperative opener, so the routing logic
// is exercised directly here.
func TestBuiltinCommandResemblance(t *testing.T) {
	d, _ := testDaemon(t)
	ponder := groundNoun(t, d, "ponder") // a known concept, not a faculty synonym
	self := d.resolveConcept("self")     // introspect's trigger, seeded at boot
	if self == nil {
		t.Fatal("the self trigger should be seeded")
	}
	feat := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "inward",
		Template: "feat:inward", Category: "feature", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(feat); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*memory.Fragment{ponder, self} {
		if err := d.store.SetLinkRel(c.ID, feat.ID, memory.RelFeature, 2.0); err != nil {
			t.Fatal(err)
		}
	}
	d.growEmbeddings()

	frame := grammar.Frame{Force: grammar.ForceCommand, Action: grammar.Term{Phrase: "ponder"}}
	line, handled := d.routeBuiltinCommand(frame, "ponder", d.chem.Snapshot())
	if !handled {
		t.Fatal("a verb resembling the introspect trigger should route there")
	}
	if want := d.introspectLine(); line != want {
		t.Errorf("ponder should run introspect: %q, want %q", line, want)
	}
}

// The router does not hijack an imperative that names no faculty and resembles
// none, leaving it to the grounded-command path.
func TestBuiltinCommandRailFallsThrough(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	frame := grammar.Frame{Force: grammar.ForceCommand, Action: grammar.Term{Phrase: "chase"}}
	if _, handled := d.routeBuiltinCommand(frame, "chase the cat", d.chem.Snapshot()); handled {
		t.Error("a non-faculty imperative should fall through, not route to a faculty")
	}
}

// A faculty command the grammar misread as a statement must not teach: its object
// is not minted as a relation, and nothing is stored.
func TestBuiltinCommandNoPollution(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "rabbit")
	say(t, d, "recollect the rabbit") // routes to recall via the fallback
	if r := d.resolveRelation("rabbit"); r != nil {
		t.Errorf("a faculty command must not mint its object as a relation: %q", r.Content)
	}
}
