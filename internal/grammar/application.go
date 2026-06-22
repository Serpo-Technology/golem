package grammar

// An operation-application call is a construction like any other: the form is the
// genitive surface "the <operation> of <arg> and <arg>...", the meaning is the
// operation applied to its operands. The grammar holds the form, parsing the
// surface into the application it names; which operation a word names, and how
// many operands it takes, is the caller's grounded concern, supplied as a
// resolver so the call reads cross-tongue through the concept graph rather than a
// word list kept here. So the same of-construction reads "the double of 4" and
// "the sum of 6 and 7", and a tongue declares its call surface in seeded words:
// the determiner, the genitive linker, the coordinator, and the word that marks a
// list literal "the list of A and B and C".

// Application is one node of an operation-application call: Head is the operation's
// canonical name as the resolver names it, Args its operands in order. A leaf
// carries no Head and a bare Lit, a value sitting where an operand does. A List
// node is the list literal "the list of A and B and C", its elements in Args,
// variadic where an operation is fixed-arity. It is the neutral structure the call
// parser yields, which the caller walks into the computation it means, so the
// grammar produces the form while the code faculty owns the meaning.
type Application struct {
	Head string        // the operation's canonical name; empty for a literal leaf or a list
	Lit  string        // the literal value, when Head is empty and List is false
	List bool          // the operands are a list literal's elements, not an operation's arguments
	Args []Application // the operands, as many as the head's arity, or the list's elements
}

// HeadFunc resolves an operation word to the operation it names: the canonical
// name the call carries and the arity that drives how many operands it reads. It
// is the grounded, cross-tongue lookup the caller supplies, so the grammar parses
// the call's form while which operation a word names stays a concern of the
// concept graph. ok is false for a word that names no operation, which fails the
// parse so the surface is read another way.
type HeadFunc func(word string) (name string, arity int, ok bool)

// ParseApplication reads the operation-application call "the <operation> of <arg>
// and <arg>..." into the application it means, the of-construction run form to
// meaning. The determiner, the genitive linker, and the coordinator are the
// tongue's seeded surface words, so a new tongue declares its call surface in
// data; the operation word and its arity come from the head resolver, so arity
// drives the nesting (a unary operand reads one argument, a binary two) and a
// nested call sits where a number would. It is all-or-nothing: the parse must
// consume every token, the surface must open a call, and every head must resolve,
// so an ill-formed or not-a-call surface returns false and the caller reads it
// another way, never a half-built tree.
func (p *Params) ParseApplication(lang, utterance string, head HeadFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	det := p.Article(lang)
	linker := p.surfaceWord(lang, "genitive")
	coord := p.Coordinator(lang)
	listMarker := p.surfaceWord(lang, "list")
	if det == "" || linker == "" || coord == "" || head == nil {
		return Application{}, false
	}
	toks, _ := Tokenize(utterance)
	app, n, ok := parseOperand(toks, det, linker, coord, listMarker, head)
	// The whole utterance must be one call and nothing left over: a leftover token
	// (a binary head handed a third operand) or a bare literal that opens no call
	// is not the call this construction reads. A list literal stands as a call too.
	if !ok || n != len(toks) || (app.Head == "" && !app.List) {
		return Application{}, false
	}
	return app, true
}

// ParseBareCall reads the bare operation call "double 4" or "add 6 and 7", the
// operation word applied straight to its arguments with none of the genitive
// scaffolding the of-construction carries: the leading word names the operation,
// resolved cross-tongue by the head callback, and its arity many operands follow,
// joined by the coordinator and each read as the of-construction reads one, so a
// nested "the double of 4" still sits where a number would. It is all-or-nothing
// like ParseApplication: the leading word must name an operation and every token
// must be consumed, so an ordinary phrase that merely opens with an operation word
// is not mistaken for a call.
func (p *Params) ParseBareCall(lang, utterance string, head HeadFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	if head == nil {
		return Application{}, false
	}
	det := p.Article(lang)
	linker := p.surfaceWord(lang, "genitive")
	coord := p.Coordinator(lang)
	listMarker := p.surfaceWord(lang, "list")
	toks, _ := Tokenize(utterance)
	// A bare call is the operation word and at least one operand; a lone word is no
	// call.
	if len(toks) < 2 {
		return Application{}, false
	}
	name, arity, ok := head(toks[0])
	if !ok || arity < 1 {
		return Application{}, false
	}
	rest := toks[1:]
	consumed := 1
	args := make([]Application, 0, arity)
	for i := 0; i < arity; i++ {
		if i > 0 {
			if len(rest) == 0 || rest[0] != coord {
				return Application{}, false
			}
			rest = rest[1:]
			consumed++
		}
		arg, n, ok := parseOperand(rest, det, linker, coord, listMarker, head)
		if !ok {
			return Application{}, false
		}
		args = append(args, arg)
		rest = rest[n:]
		consumed += n
	}
	if consumed != len(toks) {
		return Application{}, false
	}
	return Application{Head: name, Args: args}, true
}

// parseOperand reads one operand from the front of the tokens and reports how many
// it spanned: a bare token is a literal, and "the <head> of <A> and <B> and ..."
// is the operation its word names over as many operands as its arity, each read
// the same way, the recursion that lets a call nest where a value would. It fails
// on a determiner that opens no resolvable head, a missing coordinator between
// operands, or running out of tokens mid-arity, so a half-formed call does not
// half-parse.
func parseOperand(toks []string, det, linker, coord, listMarker string, head HeadFunc) (Application, int, bool) {
	if len(toks) == 0 {
		return Application{}, 0, false
	}
	// A token that does not open a call is a literal operand, a value sitting where
	// an operand does.
	if toks[0] != det {
		return Application{Lit: toks[0]}, 1, true
	}
	// The determiner opens a call only when a head and the genitive linker follow.
	if len(toks) < 3 || toks[2] != linker {
		return Application{}, 0, false
	}
	// "the list of A and B and ...": a list literal, a variadic constructor where an
	// operation is fixed-arity, so it reads operands until the coordinator runs out.
	if listMarker != "" && toks[1] == listMarker {
		consumed := 3 // "the list of"
		rest := toks[3:]
		var elems []Application
		for {
			el, n, ok := parseOperand(rest, det, linker, coord, listMarker, head)
			if !ok {
				return Application{}, 0, false
			}
			elems = append(elems, el)
			rest = rest[n:]
			consumed += n
			if len(rest) == 0 || rest[0] != coord {
				break
			}
			rest = rest[1:]
			consumed++
		}
		return Application{List: true, Args: elems}, consumed, true
	}
	// Otherwise the head names an operation of at least one operand.
	name, arity, ok := head(toks[1])
	if !ok || arity < 1 {
		return Application{}, 0, false
	}
	consumed := 3 // "the <head> of"
	rest := toks[3:]
	args := make([]Application, 0, arity)
	for i := 0; i < arity; i++ {
		if i > 0 {
			if len(rest) == 0 || rest[0] != coord {
				return Application{}, 0, false
			}
			rest = rest[1:]
			consumed++
		}
		arg, n, ok := parseOperand(rest, det, linker, coord, listMarker, head)
		if !ok {
			return Application{}, 0, false
		}
		args = append(args, arg)
		rest = rest[n:]
		consumed += n
	}
	return Application{Head: name, Args: args}, consumed, true
}
