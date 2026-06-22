package daemon

import (
	"time"

	"golem/internal/code"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// An operation the code faculty runs is seeded as a concept, named by the words that
// ask for it: its teaching word ("sum"), the operator-register word ("plus"), and its
// forms in other tongues. So which operation a computation names is resolved through
// the graph, cross-tongue and teachable, rather than a private English map kept in the
// engine, the same way a faculty's verb resolves to its command concept.

// operationWords names each arithmetic primitive beyond the "the <word> of" teaching
// surface its concept already carries: the operator-register word, the verb a bare
// imperative names the operation by ("add 6 and 7"), and a word in each other
// tongue, so a computation reads cross-tongue and the operator register is a
// grounded naming edge rather than a hardcoded table.
var operationWords = map[string]map[string][]string{
	"add":      {"en": {"plus", "add"}, "tr": {"topla", "artı"}, "la": {"adde"}},
	"subtract": {"en": {"minus", "subtract"}, "tr": {"çıkar", "eksi"}, "la": {"deme"}},
	"multiply": {"en": {"times", "multiply"}, "tr": {"çarp"}, "la": {"multiplica"}},
	"divide":   {"en": {"over", "divide"}, "tr": {"böl"}, "la": {"divide"}},
}

// ensureOperations seeds each operation the engine carries as a concept and loads the
// resolver index. Idempotent like the participants and commands: the concept and its
// naming edges are reused when present, re-minted after a consolidation prunes them.
func (d *Daemon) ensureOperations() {
	d.opConcept = map[uint64]code.OpRef{}
	if d.code == nil {
		return
	}
	for _, op := range d.code.Ops() {
		id := d.ensureOperationConcept("op:"+op.Name, op.Word)
		if id == 0 {
			continue
		}
		d.opConcept[id] = code.OpRef{Name: op.Name, Symbol: op.Symbol, Arity: op.Arity()}
		if op.Word != "" {
			d.nameConceptWord(id, op.Word, "en")
		}
		for lang, words := range operationWords[op.Name] {
			for _, w := range words {
				d.nameConceptWord(id, w, lang)
			}
		}
	}
}

// ensureOperationConcept mints an operation concept, distinct in category so a word
// that names an operation is never mistaken for a noun or a relation in ordinary talk.
// The content falls back to the name when the operation has no teaching word.
func (d *Daemon) ensureOperationConcept(template, content string) uint64 {
	if content == "" {
		content = template
	}
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "operation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		return 0
	}
	return c.ID
}

// resolveOperation maps a surface word to the operation it names, the grounded
// substitute for the engine's English word map. It resolves the word to a concept
// (cross-tongue, with case-stripping, like any other word) and returns the operation
// that concept names. This is what the engine consults through its resolver.
func (d *Daemon) resolveOperation(word string) (code.OpRef, bool) {
	c := d.resolveConcept(word)
	if c == nil {
		return code.OpRef{}, false
	}
	ref, ok := d.opConcept[c.ID]
	return ref, ok
}

// opHead resolves an operation word for the grammar's call parser: the canonical
// name the call carries and the arity that bounds its operands, both read off the
// operation's concept. It is resolveOperation in the shape the parser takes, so
// which operation a word names stays grounded and cross-tongue while the grammar
// holds only the form.
func (d *Daemon) opHead(word string) (name string, arity int, ok bool) {
	ref, ok := d.resolveOperation(word)
	if !ok {
		return "", 0, false
	}
	return ref.Name, ref.Arity, true
}

// applicationToComputation walks the application the grammar parsed into the
// computation it means, a structural copy: a node is an operation over its
// operands, a leaf a literal value. Arity was enforced as the call parsed, so the
// walk cannot fail on it.
func applicationToComputation(a grammar.Application) code.Computation {
	if a.Head == "" {
		return code.Computation{Lit: a.Lit}
	}
	args := make([]code.Computation, len(a.Args))
	for i, arg := range a.Args {
		args[i] = applicationToComputation(arg)
	}
	return code.Computation{Op: a.Head, Args: args}
}

// comprehendCall reads a computation call into its meaning, the grammar reading
// the operation-application surface ("the sum of 6 and 7") and the code engine
// the rest: an expression in operator words ("6 plus 7"), raw Go ("6 + 7"), or a
// list literal. The grammar parse is tried first and its application walked into a
// computation; when the surface opens no operation call the code engine reads it.
// So the operation a call names resolves through the concept graph, cross-tongue,
// while every surface the engine read still comprehends. This is the single point
// the daemon's computation paths comprehend a call, so the live answer and the
// learning-loop close never part on two parsers.
func (d *Daemon) comprehendCall(lang, phrase string) (code.Computation, bool) {
	if d.code == nil {
		return code.Computation{}, false
	}
	if app, ok := d.gram.ParseApplication(lang, phrase, d.opHead); ok {
		return applicationToComputation(app), true
	}
	// The bare imperative "double 4" / "add 6 and 7" names the operation with the
	// leading word and applies it to the operands that follow, the same call without
	// the genitive scaffolding.
	if app, ok := d.gram.ParseBareCall(lang, phrase, d.opHead); ok {
		return applicationToComputation(app), true
	}
	return d.code.Comprehend(phrase)
}
