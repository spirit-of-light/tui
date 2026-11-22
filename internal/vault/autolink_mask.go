package vault

import (
	"regexp"
	"sort"
	"strings"
)

// The regions below are never rewritten by autolink: a link there would break
// code, math, a table, a task or a heading, or would nest inside another link.
var (
	autolinkURLRe      = regexp.MustCompile(`(?i)(?:https?://|mailto:|www\.)\S*`)
	autolinkBareRe     = regexp.MustCompile(`<(?:[a-zA-Z][a-zA-Z0-9+.\-]*:[^<>\s]*|[^<>\s@]+@[^<>\s]+)>`)
	autolinkTagRe      = regexp.MustCompile(`</?[a-zA-Z][^<>\n]*>`)
	autolinkElementRe  = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\b[^<>\n]*>.*?</([a-zA-Z][a-zA-Z0-9]*)\s*>`)
	autolinkWikiSpanRe = regexp.MustCompile(`!?\[\[[^\]\n]*\]\]`)
	autolinkMdSpanRe   = regexp.MustCompile(`!?\[[^\]\n]*\]\([^)\n]*\)`)
	autolinkBeginRe    = regexp.MustCompile(`\\begin\{([^}\n]*)\}`)
	autolinkHeadingRe  = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]|$)`)
	autolinkTaskRe     = regexp.MustCompile(`^\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s+\[.\]`)
	autolinkCalloutRe  = regexp.MustCompile(`^\s*(?:>\s*)+\[!`)
	autolinkTableDelim = regexp.MustCompile(`^\s*\|?[\s:|\-]*-[\s:|\-]*\|?\s*$`)
)

// autolinkMask marks each byte of body that autolink must leave as it is:
// frontmatter, code, math, headings, URLs, HTML, tables, comments, task lines,
// callout titles, and the full span of each wikilink, embed and Markdown link.
// A region that is not closed runs to the end of the body.
func autolinkMask(body string) []bool {
	mask := wikiCodeMask(body)
	if _, rest, ok := Frontmatter(body); ok {
		markAll(mask[:len(body)-len(rest)])
	}
	markDelimited(mask, body, "$$", "$$")
	markDelimited(mask, body, "<!--", "-->")
	markDelimited(mask, body, "%%", "%%")
	markEnvironments(mask, body)
	for start := 0; start < len(body); {
		end := len(body)
		if i := strings.IndexByte(body[start:], '\n'); i >= 0 {
			end = start + i
		}
		maskLine(mask, body, start, end)
		start = end + 1
	}
	return mask
}

// markDelimited marks each region from an open marker to the next close marker,
// or to the end of the body. A marker that is already masked, such as one in
// code, does not open a region.
func markDelimited(mask []bool, body, open, closer string) {
	for pos := 0; pos < len(body); {
		i := strings.Index(body[pos:], open)
		if i < 0 {
			return
		}
		i += pos
		if mask[i] {
			pos = i + len(open)
			continue
		}
		end := len(body)
		if j := strings.Index(body[i+len(open):], closer); j >= 0 {
			end = i + len(open) + j + len(closer)
		}
		markAll(mask[i:end])
		pos = end
	}
}

// markEnvironments marks each `\begin{env}` to its `\end{env}`.
func markEnvironments(mask []bool, body string) {
	for pos := 0; pos < len(body); {
		m := autolinkBeginRe.FindStringSubmatchIndex(body[pos:])
		if m == nil {
			return
		}
		i := pos + m[0]
		if mask[i] {
			pos = pos + m[1]
			continue
		}
		after := pos + m[1]
		endTag := `\end{` + body[pos+m[2]:pos+m[3]] + `}`
		end := len(body)
		if j := strings.Index(body[after:], endTag); j >= 0 {
			end = after + j + len(endTag)
		}
		markAll(mask[i:end])
		pos = end
	}
}

// maskLine marks the parts of one line that are not prose.
func maskLine(mask []bool, body string, start, end int) {
	line := body[start:end]
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	switch {
	case autolinkHeadingRe.MatchString(line),
		strings.HasPrefix(strings.TrimLeft(line, " \t"), "|"),
		strings.Contains(line, "|") && autolinkTableDelim.MatchString(line),
		autolinkTaskRe.MatchString(line),
		autolinkCalloutRe.MatchString(line):
		markAll(mask[start:end])
		return
	}
	markInlineMath(mask, body, start, end)
	for _, re := range []*regexp.Regexp{autolinkURLRe, autolinkBareRe, autolinkTagRe, autolinkWikiSpanRe, autolinkMdSpanRe} {
		for _, m := range re.FindAllStringIndex(line, -1) {
			markAll(mask[start+m[0] : start+m[1]])
		}
	}
	for _, m := range autolinkElementRe.FindAllStringSubmatchIndex(line, -1) {
		if strings.EqualFold(line[m[2]:m[3]], line[m[4]:m[5]]) {
			markAll(mask[start+m[0] : start+m[1]])
		}
	}
}

// markInlineMath marks `$...$` on one line. The opening `$` has no space after
// it. The closing `$` has no space before it and no digit after it. Thus the
// text "costs $5 and X costs $10" holds no math.
func markInlineMath(mask []bool, body string, start, end int) {
	for i := start; i < end; i++ {
		if body[i] != '$' || mask[i] || isEscaped(body, start, i) {
			continue
		}
		if i+1 >= end || body[i+1] == ' ' || body[i+1] == '\t' || body[i+1] == '$' {
			continue
		}
		for j := i + 2; j < end; j++ {
			if body[j] != '$' || mask[j] || isEscaped(body, start, j) {
				continue
			}
			if body[j-1] == ' ' || body[j-1] == '\t' {
				continue
			}
			if j+1 < end && body[j+1] >= '0' && body[j+1] <= '9' {
				continue
			}
			markAll(mask[i : j+1])
			i = j
			break
		}
	}
}

// isEscaped says whether an odd number of backslashes come before body[i].
func isEscaped(body string, lineStart, i int) bool {
	n := 0
	for k := i - 1; k >= lineStart && body[k] == '\\'; k-- {
		n++
	}
	return n%2 == 1
}

// autolinkLink is a link that is already in a body: a wikilink or a Markdown
// link, with the byte span of the whole link.
type autolinkLink struct {
	Start, End int
	Wiki       bool
	Embed      bool
	Target     string // the note name, or the Markdown destination
	Anchor     string // the `#` or `^` part of a wikilink, with its leading mark
	Text       string // the shown text: the alias, the label, or the target
}

// autolinkLinks lists the links of body outside frontmatter and code, in
// order. A link in a table cell or a task line is still listed.
func autolinkLinks(body string) []autolinkLink {
	mask := wikiCodeMask(body)
	if _, rest, ok := Frontmatter(body); ok {
		markAll(mask[:len(body)-len(rest)])
	}
	var links []autolinkLink
	for _, m := range autolinkWikiSpanRe.FindAllStringIndex(body, -1) {
		if mask[m[0]] {
			continue
		}
		span := body[m[0]:m[1]]
		embed := strings.HasPrefix(span, "!")
		content := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(span, "!"), "[["), "]]")
		target, anchor, alias := SplitWikilinkContent(content)
		text := strings.TrimSpace(target)
		if alias != "" {
			text = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(alias, `\`), "|"))
		}
		links = append(links, autolinkLink{Start: m[0], End: m[1], Wiki: true, Embed: embed, Target: target, Anchor: anchor, Text: text})
	}
	for _, m := range autolinkMdSpanRe.FindAllStringIndex(body, -1) {
		if mask[m[0]] {
			continue
		}
		span := body[m[0]:m[1]]
		embed := strings.HasPrefix(span, "!")
		span = strings.TrimPrefix(span, "!")
		split := strings.Index(span, "](")
		links = append(links, autolinkLink{
			Start: m[0], End: m[1], Embed: embed,
			Target: strings.TrimSpace(span[split+2 : len(span)-1]),
			Text:   span[1:split],
		})
	}
	sort.Slice(links, func(i, j int) bool { return links[i].Start < links[j].Start })
	return links
}
