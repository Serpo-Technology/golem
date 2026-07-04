package daemon

import (
	"strings"
	"time"

	"golem/internal/code"
	"golem/internal/memory"
)

// A computed fact is knowledge GOLEM grounded by running its own program: the
// operation, the operands, and the value land as one auditable moment in the
// graph, its provenance an edge to the very code that produced it. What was
// computed once is remembered like anything seen, so the next asking reads
// memory before reaching for the runtime.

// computedFactWeight is the strength a computed fact's edges are laid at: the
// firmness of GOLEM's own observation, on par with a deliberate naming.
const computedFactWeight = 0.9

// computedTemplate keys a computed fact by its call, so the same operation over
// the same operands lands on one rememberable shape.
func computedTemplate(op string, args []string) string {
	return "computed:" + op + ":" + strings.Join(args, ":")
}

// ensureNumberConcept mints the concept a numeral names: a graph citizen for a
// number, named by its own numeral. The category is spared the nightly decay
// the way classes are, so a computed fact's referents stand across nights.
func (d *Daemon) ensureNumberConcept(n string) uint64 {
	tpl := "num:" + n
	if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: n,
		Template: tpl, Category: "number", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		return 0
	}
	d.nameConceptWord(c.ID, n, "en")
	return c.ID
}

// flatBinaryCall reads a computation as the flat binary call it is, when it is
// one: a known operation over two literal operands. Only such a call lands as a
// computed fact; a nested composition grounds nothing yet, its value spoken but
// not stored, until the landing shape grows to hold structure.
func flatBinaryCall(comp code.Computation) (op string, args []string, ok bool) {
	if comp.Op == "" || len(comp.Args) != 2 || comp.Cond != nil || comp.Map != nil {
		return "", nil, false
	}
	for _, a := range comp.Args {
		if a.Lit == "" || a.Op != "" || a.Var != "" || a.Cond != nil || a.Map != nil {
			return "", nil, false
		}
		args = append(args, a.Lit)
	}
	return comp.Op, args, true
}

// groundComputation lands a run's finding as a walkable fact. The operands and
// the value become number concepts, the operation grounds the moment the way a
// relation grounds a telling, the value hangs on its own edge, and the program
// artifact on the provenance edge, so the fact is auditable back to the code
// that ran. The content reads in the written register ("6 + 7 = 13").
func (d *Daemon) groundComputation(comp code.Computation, value string, artifactID uint64) {
	op, args, ok := flatBinaryCall(comp)
	if !ok || value == "" {
		return
	}
	opC, err := d.store.ConceptByTemplate("op:" + op)
	if err != nil || opC == nil {
		return
	}
	numA, numB, numV := d.ensureNumberConcept(args[0]), d.ensureNumberConcept(args[1]), d.ensureNumberConcept(value)
	if numA == 0 || numB == 0 || numV == 0 {
		return
	}
	surface := op
	if ref, ok := d.opConcept[opC.ID]; ok && ref.Symbol != "" {
		surface = ref.Symbol
	}
	fact := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "code",
		Content:  args[0] + " " + surface + " " + args[1] + " = " + value,
		Template: computedTemplate(op, args), Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(fact); err != nil {
		return
	}
	d.store.SetLinkRel(opC.ID, fact.ID, memory.RelGrounds, computedFactWeight)
	d.store.SetLinkRel(fact.ID, numA, memory.RelAgent, computedFactWeight)
	d.store.SetLinkRel(fact.ID, numB, memory.RelPatient, computedFactWeight)
	d.store.SetLinkRel(fact.ID, numV, memory.RelValue, computedFactWeight)
	if artifactID != 0 {
		d.store.SetLinkRel(fact.ID, artifactID, memory.RelComputedBy, computedFactWeight)
	}
}

// computedFact answers a call from memory: the fact a prior run landed for this
// operation over these operands, read back through its value edge. ok is false
// when no such fact stands, so the caller composes and runs.
func (d *Daemon) computedFact(comp code.Computation) (string, bool) {
	op, args, ok := flatBinaryCall(comp)
	if !ok {
		return "", false
	}
	f, err := d.store.FragmentByTemplate(computedTemplate(op, args))
	if err != nil || f == nil {
		return "", false
	}
	vals, err := d.store.LinksFromRel([]uint64{f.ID}, memory.RelValue)
	if err != nil || len(vals) == 0 {
		return "", false
	}
	frags, err := d.store.FragmentsByIDs([]uint64{vals[0].To})
	if err != nil || len(frags) != 1 || frags[0].Content == "" {
		return "", false
	}
	return frags[0].Content, true
}
