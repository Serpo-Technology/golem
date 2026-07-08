package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// The grammar engine is the innate floor and lives in code; its data
// lives in the graph. Each parameter (a function word and its category,
// a verb and its principal parts, a suffix and its recipe, the
// word-order setting) is a grammar node in the same store as everything
// else, readable by dump and recall, seeded at full weight on first run
// through the same encode path every memory takes. Grammar nodes sit
// outside the nightly fade entirely: native grammar is the hardest
// thing to lose, and the only force that moves it is the competition
// below.
//
// The competition is how grammar bends to real usage with no special
// case: a closed-class form that the lexicon comes to claim (its word
// nodes accruing real naming mass through deliberate labeling) erodes
// the seeded node's weight night by night, and the compile drops a
// parameter whose weight has fallen under the lived mass. Usage that
// stops recovers the seed just as gradually.
const (
	grammarErosion     = 0.10 // nightly cost of sustained contrary usage
	grammarRecovery    = 0.02 // nightly healing once the usage fades
	grammarFloor       = 0.05
	grammarContestMass = 0.30 // naming mass below this is noise, no contest
)

// taughtConstructionWeight is the entrenchment a freshly taught
// construction takes: above the seeds, so a new way to say a thing is the
// one that speaks, until it is forgotten or out-weighed.
const taughtConstructionWeight = 1.5

// loadGrammar reads the grammar parameters and the sentence constructions
// out of memory, training both curricula into starter memory on first
// run, and compiles the working copy the engine reads. The compile is the
// cache: per-parse lookups are RAM map hits, rebuilt at wake after the
// nightly competition. A memory trained by an earlier build tops up:
// innate forms and constructions the curriculum has since gained are
// seeded in, while every parameter the store already holds keeps its
// learned weight.
func (d *Daemon) loadGrammar() error {
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	if _, err := d.seedParams(gnodes, memory.KindGrammar, grammar.Curriculum()); err != nil {
		return err
	}
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	if _, err := d.seedParams(cnodes, memory.KindConstruction, grammar.Constructions()); err != nil {
		return err
	}
	// The category refinements (a verb's discourse move, a relator's direction, a
	// predicate's property) are graph concepts the words link to; ensure them and
	// the links before the compile, which derives each word's refined category from
	// the graph rather than a seeded category string.
	if err := d.ensureCategoryRefinements(); err != nil {
		return err
	}
	// The bare grammatical classes (a determiner, ...) are graph concepts a closed-class
	// word is a member of; ensure them and the links here too, before the compile, so a
	// member word's category is derived from the graph rather than read from the seed.
	if err := d.ensureCategoryClasses(); err != nil {
		return err
	}
	// The comparative scaffolding (the comparison operator, the greater and lesser pole
	// roles, and the rule, regular, irregular, and exception meta concepts) lives as class
	// concepts too; seed them here so gradable comparison is graph data rather than a Go
	// flag. The gradable adjectives and their scales are grown content, taught later.
	if err := d.ensureComparativeScaffold(); err != nil {
		return err
	}
	// The argument-structure constructions live as concepts carrying their form pole in the graph;
	// ensure them and their slot subgraphs here too, before the compile, so the derive can shadow
	// the graph representation against the compiled constructions.
	if err := d.ensureArgConstructions(); err != nil {
		return err
	}
	// The declarative grammar's phrase rules live as concepts carrying their form pole in the graph;
	// ensure them and their slot subgraphs here too, before the compile, so the derive reads each rule's
	// daughter order and agreement from the graph rather than the seed string.
	if err := d.ensureDeclGrammar(); err != nil {
		return err
	}
	// The answer templates Compose fills live as concepts carrying their template in the graph; ensure
	// them here too, before the compile, so the derive reads each template from the graph.
	if err := d.ensureAnswerConstructions(); err != nil {
		return err
	}
	// The born-in colour words name their colour cells in the graph, so colour naming reads
	// the graph like any learned word and a taught colour term is a synonym co-referring to
	// the same cell rather than an isolated token.
	if err := d.ensureColourSenses(); err != nil {
		return err
	}
	// recompileGrammar reloads both kinds from the store, so the nodes
	// just seeded are picked up without a second fetch here.
	return d.recompileGrammar()
}

// seedParams writes the missing entries of a curriculum into starter
// memory through the standard encode path, so the grammar seed and the
// construction seed are both first curricula and the seeding path is the
// one all bootstrapping uses. Encode stamps every node at maximal weight.
// Entries already held are left alone, which makes the same pass both the
// first boot and the top-up. The kind decides which curriculum a node
// belongs to; both ride the same columns.
func (d *Daemon) seedParams(held []*memory.Fragment, kind string, curriculum []grammar.Param) (int, error) {
	have := make(map[string]*memory.Fragment, len(held))
	for _, n := range held {
		have[n.Template] = n
	}
	trained := 0
	for _, p := range curriculum {
		if n := have[p.Key]; n != nil {
			// A held entry keeps its lived weight, but a closed-class word's category is
			// structural seed data, not learned, so a re-class in the seed is reconciled
			// onto the stored node at boot. Without it, a word the seed moves to a new
			// class keeps its old one on every brain saved before the move.
			if p.Category != "" && n.Category != p.Category {
				if err := d.store.UpdateCategory(n.ID, p.Category); err != nil {
					return trained, err
				}
			}
			continue
		}
		f := &memory.Fragment{
			Kind:     kind,
			Channel:  "grammar",
			Content:  p.Value,
			Template: p.Key,
			Lang:     p.Lang,
			Category: p.Category,
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(f); err != nil {
			return trained, err
		}
		trained++
	}
	return trained, nil
}

// recompileGrammar rebuilds the engine's working copy from the grammar
// nodes and the construction nodes the store now holds, each grammar
// form's lived naming mass standing against its seeded weight. Both the
// boot path and the nightly competition recompile through here, so the
// constructions are never dropped when the grammar re-weights.
func (d *Daemon) recompileGrammar() error {
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	mass, err := d.store.NamesMassByForm()
	if err != nil {
		return err
	}
	// The grammar nodes carry the closed class, the constructions' siblings,
	// and the declarative grammar the general engine runs; each compile step
	// reads the slice it understands and ignores the rest.
	gparams := paramsOf(gnodes)
	d.deriveCategoryClasses(gnodes, gparams)
	d.deriveCategoryRefinements(gnodes, gparams)
	// Antonymy between two pole adjectives is a derived fast index over the ground truth
	// (the scale plus the pole roles), rebuilt every compile from same-scale-opposite-role,
	// so a reader that wants the opposite pole gets it in one hop without a second store to
	// drift against. It is a no-op until a scale is taught.
	d.deriveRelOpposite()
	// The declarative grammar's phrase rules read their form pole (daughter order, agreement flows, head)
	// from the concepts the graph holds for them, so the general realization engine compiles the rules
	// from the graph rather than the seed string; a rule with no concept keeps its stored value.
	d.deriveDeclGrammar(gnodes, gparams)
	// A function word's inflected forms ride on its own node; the lex parameters read their category and
	// forms from there, so a word's realization morphology is sourced from the word's node in the graph.
	d.deriveDeclLex(gnodes, gparams)
	// A phrase rule taught into the graph as a concept with no grammar node is enumerated in here, so a
	// new declarative grammar rule reaches the realization engine with no seed edit.
	gparams = d.enumerateGraphDeclRules(gnodes, gparams)
	cparams := paramsOf(cnodes)
	// The argument-structure constructions' form poles are derived from the concepts the graph holds
	// for them, so the engine reads the slots, their roles, and the predicate from the graph rather
	// than the seed string; a construction with no concept (an induced per-verb one) keeps its stored
	// value. A grammar node and its compiled param line up by index, as the construction nodes do.
	d.deriveArgConstructions(cnodes, cparams)
	// A construction taught into the graph as a concept with no construction node is enumerated in
	// here, so a new argument-structure pattern reaches the engine with no seed edit.
	cparams = d.enumerateGraphArgConstructions(cnodes, cparams)
	// The answer templates Compose fills read their template and chemistry condition from the concepts
	// the graph holds for them; a template taught into the graph with no construction node is enumerated
	// in too, so a new answer realization reaches Compose with no seed edit.
	d.deriveAnswerConstructions(cnodes, cparams)
	cparams = d.enumerateGraphAnswerConstructions(cnodes, cparams)
	d.gram = grammar.Compile(gparams, mass).
		SetConstructions(cparams).
		SetDecl(gparams).
		SetArgConstructions(cparams)
	// The order-defense the role rescue consults: a filler that has already
	// held a role on record keeps the seat the teller gave it, read off the
	// graph as a presence, never a count.
	d.gram.RoleHeld = d.roleHeld
	d.code = code.NewEngine(codeOps(cparams))
	// Which word names an operation is resolved through the grounded concept graph,
	// not the engine's own English word map: the engine consults this resolver, which
	// reads the operation concepts (cross-tongue, teachable). The concepts themselves
	// are seeded by ensureOperations, called where the chemistry is ready (with the
	// participants, and after a function is learned), so this stays free of it.
	d.code.SetResolver(d.resolveOperation)
	// The body of a taught function is read through the grammar too, the daemon's
	// grounded reading cross-checked against the engine's own; re-set here since the
	// engine is rebuilt above.
	d.code.SetBodyParser(d.parseBodyGrammar)
	return nil
}

// deriveRelOpposite rebuilds the RelOpposite fast index between the pole adjectives of
// every scale: for each dimension it gathers the poles hung on it by RelScale and links
// each greater pole to each lesser pole both ways. The scale membership and the un-gated
// pole roles are the one source of truth; this index is derived from them so a describe
// or contrast reader gets the opposite pole in one hop, self-healing every boot and every
// night. It is idempotent (SetLinkRel refreshes) and a no-op before any scale is taught.
func (d *Daemon) deriveRelOpposite() {
	concepts, err := d.store.Concepts()
	if err != nil {
		return
	}
	for _, dim := range concepts {
		poles, err := d.store.LinksToRel([]uint64{dim.ID}, memory.RelScale)
		if err != nil || len(poles) < 2 {
			continue
		}
		ids := make([]uint64, 0, len(poles))
		for _, e := range poles {
			ids = append(ids, e.From)
		}
		frags, err := d.store.FragmentsByIDs(ids)
		if err != nil {
			continue
		}
		var greater, lesser []uint64
		for _, p := range frags {
			if sign, ok := d.poleSign(p); ok {
				if sign == "gt" {
					greater = append(greater, p.ID)
				} else {
					lesser = append(lesser, p.ID)
				}
			}
		}
		for _, g := range greater {
			for _, l := range lesser {
				d.store.SetLinkRel(g, l, memory.RelOpposite, 1)
				d.store.SetLinkRel(l, g, memory.RelOpposite, 1)
			}
		}
	}
}

// deriveCategoryClasses sets each closed-class word's whole category from the bare class
// it is linked to in the graph: "det" for a word marked a determiner. So the broad class
// the parser reads is derived from a graph concept rather than a seeded class string, and
// which class a word belongs to becomes a teachable link. It runs before the refinement
// derive, so a class sets the base and a refinement appends to it. A grammar node and its
// compiled param line up by index (paramsOf preserves order). While the seed still carries
// the class, a derived class that disagrees with the seeded one is logged as a tripwire that
// the links and the seed have parted; the link is authoritative.
func (d *Daemon) deriveCategoryClasses(gnodes []*memory.Fragment, gparams []grammar.Param) {
	if len(d.categoryClasses) == 0 {
		return
	}
	ids := make([]uint64, len(gnodes))
	at := make(map[uint64]int, len(gnodes))
	for i, n := range gnodes {
		ids[i] = n.ID
		at[n.ID] = i
	}
	edges, err := d.store.LinksFromRel(ids, memory.RelMarks)
	if err != nil {
		return
	}
	for _, e := range edges {
		class, ok := d.categoryClasses[e.To]
		if !ok {
			continue
		}
		i, ok := at[e.From]
		if !ok {
			continue
		}
		if prev := gparams[i].Category; prev != "" && prev != class {
			fmt.Fprintf(d.out, "category class mismatch: %q seeded %q, derived %q\n", gnodes[i].Content, prev, class)
		}
		gparams[i].Category = class
	}
}

// deriveCategoryRefinements rewrites each closed-class word's category with the
// refinement it is linked to in the graph: "speech:clarify" for a verb marking
// the clarify move, "rel:after" for a relator marking the after direction,
// "property:transitive" for a predicate marking transitivity. So the colon-refined
// category the parser reads is derived from the graph concepts rather than a
// seeded colon-suffixed string. A grammar node and its compiled param line up by
// index (paramsOf preserves order), so the refinement read off a word's node is
// written onto its param, keeping the broad class and replacing the refinement. A
// word with no refinement link keeps its bare category, the honest gap. A marks
// edge to a non-refinement concept (a deictic participant) is left alone.
func (d *Daemon) deriveCategoryRefinements(gnodes []*memory.Fragment, gparams []grammar.Param) {
	if len(d.refinements) == 0 {
		return
	}
	ids := make([]uint64, len(gnodes))
	at := make(map[uint64]int, len(gnodes))
	for i, n := range gnodes {
		ids[i] = n.ID
		at[n.ID] = i
	}
	edges, err := d.store.LinksFromRel(ids, memory.RelMarks)
	if err != nil {
		return
	}
	for _, e := range edges {
		ref, ok := d.refinements[e.To]
		if !ok {
			continue
		}
		i, ok := at[e.From]
		if !ok {
			continue
		}
		base := gparams[i].Category
		if c := strings.IndexByte(base, ':'); c >= 0 {
			base = base[:c]
		}
		gparams[i].Category = base + ":" + ref
	}
}

// codeOps reads the code constructions out of the construction parameters,
// splitting the seeded primitives from the functions learned since. An arithmetic
// primitive value is "<symbol> <word>", the operator and the word it is taught by; a
// string primitive is "str:<word> <inCSV> <out> <emit>", symbol-less and carrying its
// operand types, result type, and import-free emit template instead; a learned function
// is "func:<param,...> <body as code>", its parameters comma-joined ahead of the body,
// which the engine reads back into the operation it names so the function returns after
// a restart.
func codeOps(params []grammar.Param) ([]code.Op, []code.Def) {
	var prims []code.Op
	var defs []code.Def
	for _, p := range params {
		name, ok := strings.CutPrefix(p.Key, "code:")
		if !ok {
			continue
		}
		if spec, ok := strings.CutPrefix(p.Value, "str:"); ok {
			// A string primitive: "str:<word> <inCSV> <out> <emit>", every field
			// space-free (the operand types are comma-joined, the emit template carries
			// no space), so the four fields split cleanly.
			f := strings.Fields(spec)
			if len(f) != 4 {
				continue
			}
			prims = append(prims, code.Op{Name: name, Word: f[0],
				In: strings.Split(f[1], ","), Out: f[2], Emit: f[3]})
			continue
		}
		if spec, ok := strings.CutPrefix(p.Value, "func:"); ok {
			plist, body, ok := strings.Cut(spec, " ")
			if !ok || plist == "" || strings.TrimSpace(body) == "" {
				continue
			}
			// Each parameter is "name" (a number) or "name:list" (a list); split the
			// type off so the function reloads with its parameter types.
			raw := strings.Split(plist, ",")
			pnames := make([]string, len(raw))
			ptypes := make([]string, len(raw))
			for i, rp := range raw {
				n, t, has := strings.Cut(rp, ":")
				pnames[i] = n
				if has {
					ptypes[i] = t
				} else {
					ptypes[i] = "int"
				}
			}
			defs = append(defs, code.Def{Name: name, Params: pnames, ParamTypes: ptypes, Body: strings.TrimSpace(body)})
			continue
		}
		fields := strings.Fields(p.Value)
		if len(fields) != 2 {
			continue
		}
		prims = append(prims, code.Op{Name: name, Symbol: fields[0], Word: fields[1]})
	}
	return prims, defs
}

// paramsOf reads a set of grammar or construction nodes back into the
// parameters the engine compiles from.
func paramsOf(nodes []*memory.Fragment) []grammar.Param {
	params := make([]grammar.Param, 0, len(nodes))
	for _, n := range nodes {
		params = append(params, grammar.Param{
			Key:      n.Template,
			Value:    n.Content,
			Lang:     n.Lang,
			Category: n.Category,
			Weight:   n.Weight,
		})
	}
	return params
}

// teachConstruction mints a new way to say an intent: a construction node
// whose template the planner will fill, entrenched above the seeds so it
// is the one that speaks. The variant key is made unique per intent, so a
// second teaching of the same intent adds an alternative rather than
// overwriting the first. It is the construction twin of the mental-teach.
func (d *Daemon) teachConstruction(intent, template, lang string) error {
	if lang == "" {
		lang = "en"
	}
	intent = strings.TrimSpace(intent)
	template = strings.TrimSpace(template)
	if intent == "" || template == "" {
		return fmt.Errorf("teach --say needs an intent and a template")
	}
	if strings.ContainsAny(intent, " :") {
		return fmt.Errorf("an intent is one word, not %q", intent)
	}
	prefix := "cxn:" + lang + ":" + intent + ":"
	existing, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	n := 0
	for _, e := range existing {
		if strings.HasPrefix(e.Template, prefix+"taught") {
			n++
		}
	}
	key := fmt.Sprintf("%staught%d", prefix, n)
	node := &memory.Fragment{
		Kind:     memory.KindConstruction,
		Channel:  "grammar",
		Content:  template,
		Template: key,
		Lang:     lang,
		Time:     time.Now(),
	}
	id, err := d.store.Encode(node)
	if err != nil {
		return err
	}
	if err := d.store.SetWeight(id, taughtConstructionWeight); err != nil {
		return err
	}
	d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: taughtConstructionWeight})
	return d.recompileGrammar()
}

// teachInflection induces an inflection pattern from example (base, inflected) pairs and
// stores it as a grammar node, so a taught morphology is a fluid weight that survives a
// restart and recompiles into the working grammar. Re-teaching a feature replaces its rule
// rather than duplicating the node, since a language has one rule per feature.
func (d *Daemon) teachInflection(lang, feature string, pairs [][2]string) error {
	if lang == "" {
		lang = "en"
	}
	feature = strings.TrimSpace(feature)
	if feature == "" {
		return fmt.Errorf("teach --inflect needs a feature")
	}
	if strings.ContainsAny(feature, " :") {
		return fmt.Errorf("a feature is one word, not %q", feature)
	}
	value, ok := grammar.InduceInflection(pairs)
	if !ok {
		return fmt.Errorf("no single inflection rule fits these examples")
	}
	key := "infl:" + lang + ":" + feature
	nodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if n.Template == key {
			if err := d.store.UpdateContent(n.ID, value, key); err != nil {
				return err
			}
			d.event(events.Event{Ev: "grammar", ID: n.ID, Tpl: key, Weight: n.Weight})
			return d.recompileGrammar()
		}
	}
	node := &memory.Fragment{
		Kind:     memory.KindGrammar,
		Channel:  "grammar",
		Content:  value,
		Template: key,
		Lang:     lang,
		Time:     time.Now(),
	}
	id, err := d.store.Encode(node)
	if err != nil {
		return err
	}
	if err := d.store.SetWeight(id, taughtConstructionWeight); err != nil {
		return err
	}
	d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: taughtConstructionWeight})
	return d.recompileGrammar()
}

// parseInflectPairs reads the "base:inflected" examples a teaching carries into the pairs the
// induction takes, requiring at least two so a single example cannot masquerade as a rule.
func parseInflectPairs(toks []string) ([][2]string, error) {
	if len(toks) < 2 {
		return nil, fmt.Errorf("teach --inflect needs at least two base:inflected examples")
	}
	pairs := make([][2]string, 0, len(toks))
	for _, t := range toks {
		base, infl, ok := strings.Cut(t, ":")
		if !ok || base == "" || infl == "" {
			return nil, fmt.Errorf("each example is base:inflected, not %q", t)
		}
		pairs = append(pairs, [2]string{base, infl})
	}
	return pairs, nil
}

// grammarCompetition is the nightly weighted competition between seeded
// parameters and lived usage, run during consolidation: a closed-class
// form whose lexical naming mass is real erodes its grammar node, a
// form the lexicon has let go heals back toward full entrenchment, and
// the working grammar recompiles from what is left standing.
func (d *Daemon) grammarCompetition() {
	nodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		fmt.Fprintf(d.out, "grammar competition error: %v\n", err)
		return
	}
	mass, err := d.store.NamesMassByForm()
	if err != nil {
		fmt.Fprintf(d.out, "grammar competition error: %v\n", err)
		return
	}
	for _, n := range nodes {
		kind, _, _ := strings.Cut(n.Template, ":")
		if kind != "fw" && kind != "sortal" && kind != "hap" {
			continue // only forms compete with the lexicon
		}
		w := n.Weight
		switch {
		case mass[n.Lang+":"+n.Content] >= grammarContestMass:
			w -= grammarErosion
			if w < grammarFloor {
				w = grammarFloor
			}
		case n.Weight < 1.0:
			w += grammarRecovery
			if w > 1.0 {
				w = 1.0
			}
		default:
			continue
		}
		if w == n.Weight {
			continue
		}
		n.Weight = w
		if err := d.store.SetWeight(n.ID, w); err != nil {
			fmt.Fprintf(d.out, "grammar re-weight error: %v\n", err)
			continue
		}
		d.event(events.Event{Ev: "grammar", ID: n.ID, Tpl: n.Template, Weight: w})
	}
	if err := d.recompileGrammar(); err != nil {
		fmt.Fprintf(d.out, "grammar compile error: %v\n", err)
	}
}
