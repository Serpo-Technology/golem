package code

// Reading source is the other half of the code faculty: where Comprehend reads
// one expression into its computation, this reads a whole file into its
// declarations, so GOLEM can look at code, its own included, and reason from
// what is there. The language's own parser does the reading, the innate syntax
// floor, and the meaning kept, what each declaration is and shows, is GOLEM's.
// A change proposed against a declaration that was read and located is born of
// understanding the source rather than guessed at by editing text blind.

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// Decl is a grounded reading of one top-level declaration: what kind of thing it
// is, its name, the doc a person left on it, and a short rendering of its shape,
// a function's signature or a constant's value.
type Decl struct {
	Kind   string // func, const, var, type, import
	Name   string
	Doc    string
	Detail string
}

// Describe renders a declaration as literal language, the reading said back, so
// shown a piece of source GOLEM can say what each declaration is.
func (d Decl) Describe() string {
	switch d.Kind {
	case "func":
		return "the function " + d.Name + " has signature " + d.Detail
	case "const":
		return "the constant " + d.Name + " is " + d.Detail
	case "var":
		return "the variable " + d.Name + " is " + d.Detail
	case "type":
		return "the type " + d.Name + " is " + d.Detail
	case "import":
		return "an import of " + d.Name
	default:
		return d.Kind + " " + d.Name
	}
}

// Declarations reads a Go source file into its top-level declarations. A file
// that does not parse fails cleanly rather than yielding a partial reading.
func Declarations(src string) ([]Decl, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []Decl
	for _, d := range file.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			out = append(out, Decl{Kind: "func", Name: decl.Name.Name,
				Doc: docText(decl.Doc), Detail: render(fset, decl.Type)})
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				out = append(out, specDecls(fset, decl, spec)...)
			}
		}
	}
	return out, nil
}

// specDecls reads one spec of a const, var, type, or import block into its
// declarations, one per name a value spec binds.
func specDecls(fset *token.FileSet, decl *ast.GenDecl, spec ast.Spec) []Decl {
	doc := docText(decl.Doc)
	switch s := spec.(type) {
	case *ast.ValueSpec:
		kind := "var"
		if decl.Tok == token.CONST {
			kind = "const"
		}
		if s.Doc != nil {
			doc = docText(s.Doc)
		}
		var out []Decl
		for i, name := range s.Names {
			detail := ""
			switch {
			case i < len(s.Values):
				detail = render(fset, s.Values[i])
			case s.Type != nil:
				detail = render(fset, s.Type)
			}
			out = append(out, Decl{Kind: kind, Name: name.Name, Doc: doc, Detail: detail})
		}
		return out
	case *ast.TypeSpec:
		if s.Doc != nil {
			doc = docText(s.Doc)
		}
		return []Decl{{Kind: "type", Name: s.Name.Name, Doc: doc, Detail: render(fset, s.Type)}}
	case *ast.ImportSpec:
		return []Decl{{Kind: "import", Name: strings.Trim(s.Path.Value, `"`)}}
	}
	return nil
}

// ConstValue reads the current value of a named top-level constant out of
// source, so a change can be reasoned from what the constant is now.
func ConstValue(src, name string) (string, error) {
	lo, hi, err := constValueSpan(src, name)
	if err != nil {
		return "", err
	}
	return src[lo:hi], nil
}

// SetConst rewrites the value of a named top-level constant and returns the new
// source. The declaration is located in the parsed tree and only its value text
// is spliced, so every other byte, formatting and comments alike, is preserved;
// the result is re-parsed so a value that would not compile is refused here
// rather than later. The constant has to be there, with a value, or the change
// is declined.
func SetConst(src, name, newValue string) (string, error) {
	lo, hi, err := constValueSpan(src, name)
	if err != nil {
		return "", err
	}
	out := src[:lo] + newValue + src[hi:]
	if _, err := parser.ParseFile(token.NewFileSet(), "source.go", out, 0); err != nil {
		return "", fmt.Errorf("retuned source would not parse: %w", err)
	}
	return out, nil
}

// constValueSpan locates the byte span of a named top-level constant's value in
// the source, the offsets the reader and the rewriter both work from.
func constValueSpan(src, name string) (int, int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", src, parser.ParseComments)
	if err != nil {
		return 0, 0, err
	}
	lo, hi := -1, -1
	for _, d := range file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, id := range vs.Names {
				if id.Name == name && i < len(vs.Values) {
					lo = fset.Position(vs.Values[i].Pos()).Offset
					hi = fset.Position(vs.Values[i].End()).Offset
				}
			}
		}
	}
	if lo < 0 {
		return 0, 0, fmt.Errorf("no const %q with a value in source", name)
	}
	return lo, hi, nil
}

// render prints a syntax node back to source text, the shape of a declaration
// said in the language it is written in.
func render(fset *token.FileSet, node ast.Node) string {
	var b strings.Builder
	if err := format.Node(&b, fset, node); err != nil {
		return ""
	}
	return b.String()
}

// docText is the doc comment of a declaration as plain text, empty when there is
// none.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}
