package daemon

import (
	"time"

	"golem/internal/code"
	"golem/internal/memory"
)

// An operation the code faculty runs is seeded as a concept, named by the words that
// ask for it: its teaching word ("sum"), the operator-register word ("plus"), and its
// forms in other tongues. So which operation a computation names is resolved through
// the graph, cross-tongue and teachable, rather than a private English map kept in the
// engine, the same way a faculty's verb resolves to its command concept.

// operationWords names each arithmetic primitive beyond the "the <word> of" teaching
// surface its concept already carries: the operator-register word, and a word in each
// other tongue, so a computation reads cross-tongue and the operator register is a
// grounded naming edge rather than a hardcoded table.
var operationWords = map[string]map[string][]string{
	"add":      {"en": {"plus"}, "tr": {"topla", "artı"}, "la": {"adde"}},
	"subtract": {"en": {"minus"}, "tr": {"çıkar", "eksi"}, "la": {"deme"}},
	"multiply": {"en": {"times"}, "tr": {"çarp"}, "la": {"multiplica"}},
	"divide":   {"en": {"over"}, "tr": {"böl"}, "la": {"divide"}},
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
