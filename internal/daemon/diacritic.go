package daemon

import (
	"strings"
	"time"
	"unicode"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// A Turkish word written in ASCII ("buyuk") is the same word seen through a lossy
// channel: the six diacritic letters have been flattened to their bare Latin twins.
// restoreDiacriticTR is the pure generator that recovers the candidate spellings by
// folding each ambiguous position to both members of its class and enumerating the
// product, in a fixed order so the candidate list is byte-stable across runs. It reads
// nothing and writes nothing; the resolver hook probes the candidates against the
// lexicon and keeps only the ones GOLEM already knows, the same generate-and-resolve
// shape as stemComparativeEN feeding comparativeToPole. It mirrors that runtime
// function, distinct from the offline-frozen morphology a rule derives once.

// trFoldClasses maps each ambiguous Turkish letter to the two members of its fold
// class. Every key enumerates BOTH members, ASCII and diacritic, so a lossy ASCII
// input, a proper diacritic input, and a mixed one are all covered by one product with
// no direction flag. The six classes are an invariant of the Turkish writing system,
// not taught and not per-brain, exactly the niche of the grammar's vowels table, so
// they live as a Go var rather than graph data or a code-faculty rule.
var trFoldClasses = map[rune][]rune{
	'u': {'u', 'ü'}, 'ü': {'u', 'ü'},
	'o': {'o', 'ö'}, 'ö': {'o', 'ö'},
	'i': {'i', 'ı'}, 'ı': {'i', 'ı'},
	'c': {'c', 'ç'}, 'ç': {'c', 'ç'},
	's': {'s', 'ş'}, 'ş': {'s', 'ş'},
	'g': {'g', 'ğ'}, 'ğ': {'g', 'ğ'},
}

// diacriticKMAX caps how many foldable positions a surface may carry: the product is
// 2^k candidates, so eight positions is 256 and a token past that returns empty rather
// than blowing up. A word with more foldable letters than this stays unresolved.
const diacriticKMAX = 8

// lowerTR lowercases in the Turkish locale before folding, where the default casing
// mis-handles the dotted and dotless I. It maps I to ı and İ to i (the Turkish
// pairing), strips a combining dot above so a decomposed İ collapses to i, and
// lowercases every other rune normally. Because the fold then enumerates the full
// {i, ı} class, a plainly cased ASCII input still reaches its diacritic form.
func lowerTR(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'I':
			b.WriteRune('ı')
		case 'İ':
			b.WriteRune('i')
		case '\u0307': // combining dot above (decomposed İ): drop so it folds as i
			continue
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// restoreDiacriticTR returns the Turkish spelling candidates a surface folds to: it
// Turkish-lowercases the input, then enumerates the cartesian product of the fold
// classes over the foldable positions, in the fixed left-to-right order the positions
// appear and, at each, the fixed order its class lists. A surface with more than
// diacriticKMAX foldable positions returns nil (the product would be too large). A
// surface with no foldable position returns just itself, which the caller has already
// probed exactly, so the miss stands.
func restoreDiacriticTR(surface string) []string {
	s := lowerTR(strings.TrimSpace(surface))
	if s == "" {
		return nil
	}
	runes := []rune(s)
	foldable := 0
	for _, r := range runes {
		if _, ok := trFoldClasses[r]; ok {
			foldable++
		}
	}
	if foldable > diacriticKMAX {
		return nil
	}
	cands := []string{""}
	for _, r := range runes {
		class, ok := trFoldClasses[r]
		if !ok {
			for i := range cands {
				cands[i] += string(r)
			}
			continue
		}
		next := make([]string, 0, len(cands)*len(class))
		for _, prefix := range cands {
			for _, v := range class {
				next = append(next, prefix+string(v))
			}
		}
		cands = next
	}
	return cands
}

// restoredNote records a surface the diacritic fold recovered this turn: the concept
// it resolved to and the confidence a bare guess of it may be spoken at. It is the
// per-turn memo (so a span folds once), the drop signal the word-form mint reads (so
// a lossy ASCII surface never earns its own KindWord row), and the anti-leak set the
// naming company reads (so no RelNames alias to the proper concept is ever born).
type restoredNote struct {
	conceptID uint64
	conf      float64
}

// restoredConf is the confidence a bare diacritic restoration is spoken at: a fixed
// value below weakHedgeConf, so hedgeWord grades it to the seeded weak "maybe" shade
// deterministically. A bare fold, with nothing in a clause to corroborate it, is a
// doubt, never a firm claim; the in-clause path grounds firmly through clause coherence
// instead, so it never reads this constant.
const restoredConf = 0.3

// foldResolve is the diacritic-restoration fallback on a resolver miss. On a
// Turkish-gated miss it folds the phrase to its candidates, keeps only the ones that
// already have a Turkish word node, DECLINES when two or more distinct candidate forms
// survive (a minimal-pair ambiguity held as a gap rather than guessed), and otherwise
// feeds the single surviving form's rows to the same warmth vote the exact path runs.
// It records the accepted restoration as a per-turn note and never mints, so a taught
// clause containing "buyuk" lands on the pre-existing "büyük" and an unresolvable ASCII
// token still returns nil and stays learnable by ordinary minting. It is re-entrant:
// the candidates are re-probed directly, not through a recursive resolve, so there is
// no fold cascade.
func (d *Daemon) foldResolve(phrase, lang string) *memory.Fragment {
	if !d.foldEnabled || lang != "tr" {
		return nil
	}
	surface := strings.ToLower(strings.TrimSpace(phrase))
	if surface == "" {
		return nil
	}
	// A span already folded this turn resolves to its recorded concept without
	// regenerating, the per-turn memo that keeps repeated resolve() calls cheap.
	if note, ok := d.restored[surface]; ok {
		if frags, err := d.store.FragmentsByIDs([]uint64{note.conceptID}); err == nil && len(frags) == 1 {
			return frags[0]
		}
		return nil
	}
	form, rows, ok := d.foldCandidateRows(surface)
	if !ok {
		return nil
	}
	f := d.voteNamed(rows, []string{form}, lang)
	if f == nil {
		return nil
	}
	if d.restored == nil {
		d.restored = map[string]restoredNote{}
	}
	d.restored[surface] = restoredNote{conceptID: f.ID, conf: restoredConf}
	return f
}

// foldCandidateRows runs the fold and the decline belt for a surface, returning the
// single surviving Turkish form and its word-node rows. The belt is keyed on distinct
// candidate FORMS (the .Content), not concept count: two forms with a node ("yasa" and
// "yaşa") decline as a minimal-pair ambiguity, while one form naming several concepts
// (ordinary polysemy) proceeds and is settled by the warmth vote. Rows are restricted
// to the Turkish tongue, so a folded restoration never pools with a same-form English
// node whatever the incoming lang.
func (d *Daemon) foldCandidateRows(surface string) (form string, rows []*memory.Fragment, ok bool) {
	cands := restoreDiacriticTR(surface)
	if len(cands) == 0 {
		return "", nil, false
	}
	words, err := d.store.WordNodesByForms(cands)
	if err != nil || len(words) == 0 {
		return "", nil, false
	}
	byForm := map[string][]*memory.Fragment{}
	var forms []string
	for _, w := range words {
		if w.Lang != "tr" {
			continue // tongue-safe accept: a Turkish fold resolves only to Turkish nodes
		}
		c := strings.ToLower(w.Content)
		if _, seen := byForm[c]; !seen {
			forms = append(forms, c)
		}
		byForm[c] = append(byForm[c], w)
	}
	if len(forms) != 1 {
		return "", nil, false // no surviving form, or an ambiguous minimal pair: decline
	}
	return forms[0], byForm[forms[0]], true
}

// diacriticFreeTR reports whether a surface carries no Turkish diacritic, i.e. it is the
// fully flattened ASCII view its lossy channel would produce. A surface holding any of the
// six diacritic letters is a deliberate spelling (one member of a minimal pair like sık or
// şık, or an already-proper form like büyük), not a channel loss to recover.
func diacriticFreeTR(s string) bool {
	for _, r := range s {
		switch r {
		case 'ü', 'ö', 'ı', 'ç', 'ş', 'ğ':
			return false
		}
	}
	return true
}

// mintWordNode is the single seam every word-form mint passes through. Before a Turkish
// surface is minted it runs the diacritic fold: a fully flattened ASCII form that recovers
// a single existing proper spelling ("buyuk" of "büyük") resolves to that node instead of
// forking a lossy shadow. This makes "buyuk and büyük are one word" hold at the mint
// itself, rather than depending on every caller resolving the surface first, so a pole or
// comparative surface minted before its span was framed can no longer leak an orphan. The
// fold fires only for a fully flattened surface: a form still carrying a diacritic is a
// deliberate spelling and mints its own node, so a minimal pair (sık, şık) stays two words.
// A non-Turkish surface, a surface that folds to nothing, and one that folds to an ambiguous
// pair all mint as usual. Unlike foldResolve it is not gated on foldEnabled, because a mint
// never runs inside the language sniff, so the teaching commands that skip the tell-path
// resolve are covered too.
func (d *Daemon) mintWordNode(form, lang string, chem limbic.State, at time.Time) (*memory.Fragment, bool, error) {
	if lang == "tr" {
		surface := strings.ToLower(strings.TrimSpace(form))
		if diacriticFreeTR(surface) {
			if folded, rows, ok := d.foldCandidateRows(surface); ok && folded != surface && len(rows) > 0 {
				return rows[0], false, nil
			}
		}
	}
	return d.store.WordNode(form, lang, chem, at)
}

// foldOneForm returns the single Turkish word the token's diacritic fold recovers, or
// "" when nothing resolves or two or more distinct forms do (the same decline belt the
// resolver runs). It also folds the token's case stems, so an inflected bare token
// ("buyugu") restores. It is the bare-doubt seam's resolver, apart from foldResolve
// because the doubt speaks a proper form, not a concept, and fires only when nothing
// else answered.
func (d *Daemon) foldOneForm(token, lang string) string {
	if lang != "tr" {
		return ""
	}
	forms := map[string]bool{}
	stems := append([]string{token}, d.gram.CaseStems(lang, token)...)
	tried := map[string]bool{}
	for _, stem := range stems {
		if stem == "" || tried[stem] {
			continue
		}
		tried[stem] = true
		if form, _, ok := d.foldCandidateRows(strings.ToLower(strings.TrimSpace(stem))); ok {
			forms[form] = true
		}
	}
	if len(forms) != 1 {
		return ""
	}
	for f := range forms {
		return f
	}
	return ""
}

// dropRestoredWords removes the word nodes of surfaces the diacritic fold recovered
// this turn, so a lossy ASCII form never joins the naming company that would consolidate
// into a RelNames alias and collapse the doubt into an exact hit. The word-form mint is
// already suppressed for these surfaces, so in practice they are absent from the list;
// this keeps the naming sites explicitly clear of them regardless.
func (d *Daemon) dropRestoredWords(words []*memory.Fragment) []*memory.Fragment {
	if len(d.restored) == 0 {
		return words
	}
	out := make([]*memory.Fragment, 0, len(words))
	for _, w := range words {
		if _, ok := d.restored[strings.ToLower(w.Content)]; ok {
			continue
		}
		out = append(out, w)
	}
	return out
}

// isRestoredForm reports whether a surface form was recovered by the fold this turn,
// the test the word-form mint reads to skip minting a KindWord row for a lossy ASCII
// surface.
func (d *Daemon) isRestoredForm(form string) bool {
	if len(d.restored) == 0 {
		return false
	}
	_, ok := d.restored[strings.ToLower(strings.TrimSpace(form))]
	return ok
}

// restoreDoubt renders the hedged bare restoration through the existing doubt rail: the
// tongue's weak hedge (graded from restoredConf), the "you mean" carrier, and the proper
// form, shaped by the tongue (English places the carrier before the form, the verb-final
// tongues after it). It returns "" when the tongue lacks the hedge or the carrier, so the
// caller falls soft to the plain lost admission.
func (d *Daemon) restoreDoubt(lang, form string) string {
	if form == "" {
		return ""
	}
	hedge := d.hedgeWord(lang, restoredConf)
	carrier := d.gram.Word(lang, "youmean")
	if hedge == "" || carrier == "" {
		return ""
	}
	switch lang {
	case "tr", "la":
		// Turkish and Latin place the "mean" verb clause-finally: "Belki büyük demek istiyorsun?"
		return upperFirst(hedge) + " " + form + " " + carrier + "?"
	default:
		return upperFirst(hedge) + " " + carrier + " " + form + "?"
	}
}

// bareRestoreDoubt speaks the hedged restoration for an utterance that is a lone content
// token nothing could answer. It re-folds the raw token itself (independent of
// frame.Unknown and of any pre-populated restored note, both unreliable in the bare
// case), runs the decline belt, and renders the doubt only when exactly one known Turkish
// word survives. The fold is always Turkish (its safety rests on resolving to a real
// Turkish node, not on the turn's tongue), so a bare ASCII "buyuk" seen with no context,
// even on an untagged turn read as English, is guessed with doubt the way the architect's
// own reading does; the doubt is spoken in the turn's tongue ("Perhaps you mean büyük?" /
// "Belki büyük demek istiyorsun?"). It fires only when the single content token uniquely
// restores, so it is mutually exclusive with the inference hedge and never double-hedges.
func (d *Daemon) bareRestoreDoubt(lang, raw string) (string, bool) {
	content := d.gram.ContentTokensLang(lang, raw)
	if len(content) != 1 {
		return "", false
	}
	form := d.foldOneForm(content[0], "tr")
	if form == "" || form == lowerTR(strings.TrimSpace(content[0])) {
		return "", false // nothing restored, or the token was already the proper spelling
	}
	line := d.restoreDoubt(lang, form)
	if line == "" {
		return "", false
	}
	return line, true
}
