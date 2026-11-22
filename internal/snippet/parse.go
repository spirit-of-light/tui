// Package snippet parses and expands snippet bodies: text with tabstops,
// placeholders, mirrors, a final position and template variables. It imports
// only the standard library, so any part of zn can use it with no terminal,
// vault or config.
package snippet

import "strings"

// Kind tells what a Node is.
type Kind int

const (
	// KindText is literal text with the escapes already removed.
	KindText Kind = iota
	// KindTabstop is a tabstop. Its Children hold a placeholder, if any.
	KindTabstop
	// KindVariable is a template variable such as ${title} or ${date:FMT}.
	KindVariable
)

// Node is one part of a parsed body.
type Node struct {
	Kind Kind
	// Text is the literal text of a KindText node.
	Text string
	// Number is the number of a KindTabstop node. Number 0 is the final position.
	Number int
	// Children is the placeholder content of a KindTabstop node.
	Children []Node
	// Name, Arg and HasArg describe a KindVariable node. HasArg is true when
	// the source had a ":" after the name, even if the argument is empty.
	Name   string
	Arg    string
	HasArg bool
	// Raw is the source text of a KindVariable node. It stays in the output
	// when the variable has no value.
	Raw string
}

type tokKind int

const (
	tokText tokKind = iota
	tokStop
	tokOpen
	tokClose
	tokVar
)

type token struct {
	kind tokKind
	// text is the literal text of a text token, or the source of an opener
	// or a variable.
	text   string
	number int
	name   string
	arg    string
	hasArg bool
	// matched is true for an opener that has a closing brace.
	matched bool
}

// maxDigits limits the digits of a tabstop number so it always fits an int.
const maxDigits = 9

// Parse reads a snippet body. It never fails: a part that is not valid
// stays literal text, and parsing continues after it.
func Parse(body string) []Node {
	toks := lex([]rune(body))
	markMatches(toks)
	p := &parser{toks: toks}
	return p.sequence(false)
}

// lex splits the body into tokens. It does not depend on context.
func lex(src []rune) []token {
	var toks []token
	var text []rune
	flush := func() {
		if len(text) > 0 {
			toks = append(toks, token{kind: tokText, text: string(text)})
			text = text[:0]
		}
	}
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch c {
		case '\\':
			if i+1 < n && (src[i+1] == '$' || src[i+1] == '}') {
				text = append(text, src[i+1])
				i += 2
				continue
			}
			if i+2 < n && src[i+1] == '\\' && src[i+2] == '$' {
				// One literal backslash. The "$" after it stays active.
				text = append(text, '\\')
				i += 2
				continue
			}
			text = append(text, c)
			i++
		case '}':
			flush()
			toks = append(toks, token{kind: tokClose})
			i++
		case '$':
			tok, size, ok := lexDollar(src, i)
			if !ok {
				text = append(text, c)
				i++
				continue
			}
			flush()
			toks = append(toks, tok)
			i += size
		default:
			text = append(text, c)
			i++
		}
	}
	flush()
	return toks
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// number reads the maximal digit run at src[i:]. It returns the value, the
// run length, and false when the run is empty or too long.
func number(src []rune, i int) (int, int, bool) {
	j := i
	for j < len(src) && isDigit(src[j]) {
		j++
	}
	if j == i {
		return 0, 0, false
	}
	k := i
	for k < j-1 && src[k] == '0' {
		k++
	}
	if j-k > maxDigits {
		return 0, 0, false
	}
	v := 0
	for ; k < j; k++ {
		v = v*10 + int(src[k]-'0')
	}
	return v, j - i, true
}

// lexDollar reads the token that starts with the "$" at src[i]. When it
// returns false, the "$" or the "${" opener is literal text.
func lexDollar(src []rune, i int) (token, int, bool) {
	n := len(src)
	if i+1 >= n {
		return token{}, 0, false
	}
	if isDigit(src[i+1]) {
		v, size, ok := number(src, i+1)
		if !ok {
			return token{}, 0, false
		}
		return token{kind: tokStop, number: v}, 1 + size, true
	}
	if src[i+1] != '{' {
		return token{}, 0, false
	}
	j := i + 2
	if j < n && isDigit(src[j]) {
		v, size, ok := number(src, j)
		if !ok {
			return token{}, 0, false
		}
		j += size
		if j < n && src[j] == '}' {
			return token{kind: tokStop, number: v}, j + 1 - i, true
		}
		if j < n && src[j] == ':' {
			return token{kind: tokOpen, number: v, text: string(src[i : j+1])}, j + 1 - i, true
		}
		return token{}, 0, false
	}
	for _, name := range []string{"title", "date", "time", "week"} {
		end := j + len(name)
		if end > n || string(src[j:end]) != name {
			continue
		}
		if end < n && src[end] == '}' {
			return token{kind: tokVar, name: name, text: string(src[i : end+1])}, end + 1 - i, true
		}
		if name == "date" && end < n && src[end] == ':' {
			var arg []rune
			for k := end + 1; k < n; k++ {
				switch {
				case src[k] == '\\' && k+1 < n && (src[k+1] == '}' || src[k+1] == '$'):
					arg = append(arg, src[k+1])
					k++
				case src[k] == '}':
					return token{kind: tokVar, name: name, arg: string(arg), hasArg: true,
						text: string(src[i : k+1])}, k + 1 - i, true
				default:
					arg = append(arg, src[k])
				}
			}
		}
	}
	return token{}, 0, false
}

// markMatches marks each placeholder opener that has a closing brace. A
// closing brace closes the innermost open placeholder. An opener that is
// still open at the end is literal text.
func markMatches(toks []token) {
	var stack []int
	for i, t := range toks {
		switch t.kind {
		case tokOpen:
			stack = append(stack, i)
		case tokClose:
			if len(stack) > 0 {
				toks[stack[len(stack)-1]].matched = true
				stack = stack[:len(stack)-1]
			}
		}
	}
}

type parser struct {
	toks []token
	pos  int
}

// sequence reads nodes up to the closing brace of the current placeholder
// (inside is true) or to the end of the body.
func (p *parser) sequence(inside bool) []Node {
	var nodes []Node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			nodes = append(nodes, Node{Kind: KindText, Text: text.String()})
			text.Reset()
		}
	}
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		p.pos++
		switch t.kind {
		case tokText:
			text.WriteString(t.text)
		case tokClose:
			if inside {
				flush()
				return nodes
			}
			text.WriteByte('}')
		case tokStop:
			flush()
			nodes = append(nodes, Node{Kind: KindTabstop, Number: t.number})
		case tokVar:
			flush()
			nodes = append(nodes, Node{Kind: KindVariable, Name: t.name, Arg: t.arg, HasArg: t.hasArg, Raw: t.text})
		case tokOpen:
			if !t.matched {
				text.WriteString(t.text)
				continue
			}
			flush()
			children := p.sequence(true)
			nodes = append(nodes, Node{Kind: KindTabstop, Number: t.number, Children: children})
		}
	}
	flush()
	return nodes
}
