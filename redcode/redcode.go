// Package redcode assembles ICWS'94 Redcode source into mars warriors.
//
// It supports labels, EQU, ORG, END, the P-space opcodes LDP and STP, the
// ;name and ;author comments, integer expressions (+ - * / % and
// parentheses) and the predefined constants CORESIZE, MAXPROCESSES,
// MAXCYCLES, MAXLENGTH, MINDISTANCE, PSPACESIZE, WARRIORS, VERSION and
// CURLINE. PIN is accepted but ignored: P-space is never shared. FOR/ROF
// blocks with & concatenation are implemented but disabled; see ForEnabled.
//
// Expressions also accept pMARS's comparison and logical operators
// == != < > <= >= && || and !. ;assert comments are ignored: a warrior
// written for another core size runs anyway, and fares as it may.
//
// Sources in the older CWS'86/'88 style, with no commas at all, have their
// two operands separated by whitespace instead, as in "mov < 2 0".
//
// As in pMARS, if the source has a line starting ";redcode", everything
// before the first one is ignored, so warriors can be loaded straight from
// e-mail submissions. Assembly stops at END.
package redcode

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/davecheney/corewar/mars"
)

// Error is an assembly error at a source line.
type Error struct {
	Line int
	Msg  string
	Err  error // the underlying cause, if any
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

func (e *Error) Unwrap() error { return e.Err }

// Version is the value of the predefined VERSION constant, matching pMARS
// 0.9.6, so that warriors asserting a minimum pMARS version assemble.
const Version = 96

type operand struct {
	mode mars.Mode
	expr []string
}

type statement struct {
	line   int
	op     mars.Opcode
	mod    mars.Modifier
	hasMod bool
	nops   int
	ops    [2]operand
}

type assembler struct {
	cfg    mars.Config
	labels map[string]int
	equs   map[string][]string
	lines  map[string]int // line an EQU was defined on, for errors
	stmts  []statement
	org    []string
	orgAt  int
	expand int // lines produced by FOR blocks so far
	tokbuf []string
}

// ForEnabled turns on FOR/ROF blocks. They are disabled for now: a warrior
// using them is rejected.
const ForEnabled = false

// maxExpand bounds how many lines FOR blocks may produce, so a warrior
// cannot make the assembler run out of memory.
const maxExpand = 1 << 16

type srcLine struct {
	n    int // line number in the original source
	text string
}

// Assemble assembles src for a battle with the given configuration.
func Assemble(src string, cfg mars.Config) (*mars.Warrior, error) {
	a := &assembler{
		cfg:    cfg,
		labels: make(map[string]int),
		equs:   make(map[string][]string),
		lines:  make(map[string]int),
	}
	w := &mars.Warrior{}
	var pending []string

	lines := splitLines(src)
	a.stmts = make([]statement, 0, min(len(lines), cfg.MaxLength))
	commaless := !slices.ContainsFunc(lines, hasComma)
	for i := 0; i < len(lines); i++ {
		n, text := lines[i].n, lines[i].text
		if d, ok := strings.CutPrefix(strings.TrimSpace(text), "."); ok {
			d, _, _ = strings.Cut(d, " ")
			d, _, _ = strings.Cut(d, "\t")
			return nil, &Error{Line: n, Msg: fmt.Sprintf("unknown directive .%s: not ICWS'94 Redcode", d)}
		}
		text, comment, _ := strings.Cut(text, ";")
		if val, ok := strings.CutPrefix(strings.TrimSpace(comment), "name"); ok && w.Name == "" && startsSpace(val) {
			w.Name = strings.Clone(strings.TrimSpace(val))
		}
		if val, ok := strings.CutPrefix(strings.TrimSpace(comment), "author"); ok && w.Author == "" && startsSpace(val) {
			w.Author = strings.Clone(strings.TrimSpace(val))
		}
		if commaless {
			text = insertComma(text)
		}
		toks, err := a.tokenize(text)
		if err != nil {
			return nil, &Error{Line: n, Msg: err.Error()}
		}
		if len(toks) == 0 {
			continue
		}

		// Leading labels, each optionally followed by a colon.
		guess := ""
		for len(toks) > 0 && isIdent(toks[0]) && !isKeyword(toks[0]) {
			if !slices.Contains(pending, toks[0]) {
				pending = append(pending, toks[0])
			}
			label := toks[0]
			toks = toks[1:]
			if len(toks) > 0 && toks[0] == ":" {
				toks = toks[1:]
			} else if guess == "" {
				guess = label
			}
		}
		if len(toks) == 0 {
			continue
		}
		if !isIdent(toks[0]) && guess != "" {
			// A "label" with no colon that runs into an operand was more
			// likely a misspelled opcode, as in "sti r1, 2" or "mvo 0, 1".
			return nil, &Error{Line: n, Msg: fmt.Sprintf("unknown opcode %q", guess)}
		}

		switch directive(toks[0]) {
		case "FOR":
			if !ForEnabled {
				return nil, &Error{Line: n, Msg: "FOR/ROF is not supported"}
			}
			counter := ""
			if len(pending) > 0 {
				counter, pending = pending[len(pending)-1], pending[:len(pending)-1]
			}
			count, err := a.eval(toks[1:], len(a.stmts), n)
			if err != nil {
				return nil, err
			}
			end := matchROF(lines, i)
			if end < 0 {
				return nil, &Error{Line: n, Msg: "FOR without ROF"}
			}
			count = max(count, 0)
			if a.expand += count * (end - i - 1); a.expand > maxExpand {
				return nil, &Error{Line: n, Msg: "FOR blocks expand to too many lines"}
			}
			// Any other labels carry over to the first expanded line.
			lines = expandFor(lines, i, end, counter, count)
			continue
		case "ROF":
			if !ForEnabled {
				return nil, &Error{Line: n, Msg: "FOR/ROF is not supported"}
			}
			return nil, &Error{Line: n, Msg: "ROF without FOR"}
		case "PIN":
			if _, err := a.eval(toks[1:], len(a.stmts), n); err != nil {
				return nil, err
			}
			continue
		case "EQU":
			if len(pending) == 0 {
				return nil, &Error{Line: n, Msg: "EQU without a label"}
			}
			for _, l := range pending {
				if err := a.define(l, n); err != nil {
					return nil, err
				}
				a.equs[l] = toks[1:]
			}
			pending = nil
			continue
		case "ORG":
			a.org, a.orgAt = toks[1:], n
			continue
		case "END":
			if len(toks) > 1 {
				a.org, a.orgAt = toks[1:], n
			}
			for _, l := range pending {
				if err := a.define(l, n); err != nil {
					return nil, err
				}
				a.labels[l] = len(a.stmts)
			}
			pending = nil
			goto done
		}

		for _, l := range pending {
			if err := a.define(l, n); err != nil {
				return nil, err
			}
			a.labels[l] = len(a.stmts)
		}
		pending = nil

		toks, err = a.expandEQUs(toks, n)
		if err != nil {
			return nil, err
		}
		st, err := parseStatement(toks, n)
		if err != nil {
			return nil, err
		}
		a.stmts = append(a.stmts, st)
	}
done:
	if len(a.stmts) == 0 {
		return nil, &Error{Line: 0, Msg: "no instructions"}
	}
	if len(a.stmts) > cfg.MaxLength {
		return nil, &Error{Line: 0, Msg: fmt.Sprintf("%d instructions exceeds MAXLENGTH %d", len(a.stmts), cfg.MaxLength)}
	}

	w.Code = make([]mars.Instruction, 0, len(a.stmts))
	for i, st := range a.stmts {
		ins := mars.Instruction{Op: st.op, AMode: mars.Direct, BMode: mars.Direct}
		ops := st.ops[:st.nops]
		if st.op == mars.DAT && len(ops) == 1 {
			ops = []operand{{mode: mars.Immediate, expr: zeroExpr}, ops[0]}
		}
		if len(ops) > 0 {
			v, err := a.eval(ops[0].expr, i, st.line)
			if err != nil {
				return nil, err
			}
			ins.AMode, ins.A = ops[0].mode, a.fold(v)
		}
		if len(ops) > 1 {
			v, err := a.eval(ops[1].expr, i, st.line)
			if err != nil {
				return nil, err
			}
			ins.BMode, ins.B = ops[1].mode, a.fold(v)
		}
		ins.Mod = st.mod
		if !st.hasMod {
			ins.Mod = mars.DefaultModifier(ins.Op, ins.AMode, ins.BMode)
		}
		w.Code = append(w.Code, ins)
	}

	if a.org != nil {
		v, err := a.eval(a.org, 0, a.orgAt)
		if err != nil {
			return nil, err
		}
		w.Start = ((v % len(w.Code)) + len(w.Code)) % len(w.Code)
	}
	return w, nil
}

// hasComma reports whether l has a comma outside its comment.
func hasComma(l srcLine) bool {
	code, _, _ := strings.Cut(l.text, ";")
	return strings.IndexByte(code, ',') >= 0
}

// insertComma rewrites an instruction in the CWS'86/'88 style, whose
// operands are separated by whitespace, as in "mov < src 0", to separate
// them with a comma. A mode character standing alone belongs to the field
// after it. Lines without an opcode or without exactly two operand fields
// are returned unchanged.
func insertComma(text string) string {
	fields := strings.Fields(text)
	op := slices.IndexFunc(fields, func(f string) bool {
		name, _, _ := strings.Cut(f, ".")
		_, ok := lookupOpcode(name)
		return ok
	})
	if op < 0 {
		return text
	}
	var operands []string
	for i := op + 1; i < len(fields); i++ {
		f := fields[i]
		if isModeChar(f) && i+1 < len(fields) {
			i++
			f += fields[i]
		}
		operands = append(operands, f)
	}
	if len(operands) != 2 {
		return text
	}
	return strings.Join(fields[:op+1], " ") + " " + operands[0] + ", " + operands[1]
}

// splitLines splits src into numbered lines, dropping any before the first
// ";redcode" line.
func splitLines(src string) []srcLine {
	if i := redcodeLine(src); i > 0 {
		n := strings.Count(src[:i], "\n")
		return splitFrom(src[i:], n+1)
	}
	return splitFrom(src, 1)
}

// redcodeLine returns the offset of the first ";redcode" line in src, or
// -1 if there is none.
func redcodeLine(src string) int {
	for off := 0; off < len(src); {
		line, _, _ := strings.Cut(src[off:], "\n")
		if isRedcodeLine(line) {
			return off
		}
		off += len(line) + 1
	}
	return -1
}

func splitFrom(src string, n int) []srcLine {
	lines := make([]srcLine, 0, strings.Count(src, "\n")+1)
	for {
		line, rest, more := strings.Cut(src, "\n")
		lines = append(lines, srcLine{n, line})
		if !more {
			return lines
		}
		src, n = rest, n+1
	}
}

func isRedcodeLine(text string) bool {
	t := strings.TrimSpace(text)
	return len(t) >= 8 && strings.EqualFold(t[:8], ";redcode")
}

// blockKeyword returns "FOR" or "ROF" if text is a FOR or ROF line, looking
// past any labels, and "" otherwise. It works on raw text so that FOR 0
// blocks, which are often used for comments, need not be valid Redcode.
func blockKeyword(text string) string {
	code, _, _ := strings.Cut(text, ";")
	for _, f := range strings.Fields(code) {
		switch kw := directive(f); kw {
		case "FOR", "ROF":
			return kw
		}
		f = strings.TrimSuffix(f, ":")
		if f == "" || !isIdent(f) || isKeyword(f) {
			return ""
		}
		for _, c := range f {
			if !isIdentChar(byte(c)) {
				return ""
			}
		}
	}
	return ""
}

// matchROF returns the index of the ROF closing the FOR at lines[i], or -1.
func matchROF(lines []srcLine, i int) int {
	depth := 0
	for j := i + 1; j < len(lines); j++ {
		switch blockKeyword(lines[j].text) {
		case "FOR":
			depth++
		case "ROF":
			if depth == 0 {
				return j
			}
			depth--
		}
	}
	return -1
}

// expandFor replaces the FOR block lines[i+1:end+1], its body and ROF,
// with count copies of the body, splicing them in place so that nested
// blocks do not copy the whole program each time.
func expandFor(lines []srcLine, i, end int, counter string, count int) []srcLine {
	body := slices.Clone(lines[i+1 : end])
	oldLen := len(lines)
	newLen := oldLen - (end - i) + count*len(body)
	if newLen > oldLen {
		lines = slices.Grow(lines, newLen-oldLen)[:newLen]
	}
	copy(lines[i+1+count*len(body):], lines[end+1:oldLen])
	lines = lines[:newLen]
	at := i + 1
	for k := 1; k <= count; k++ {
		for _, l := range body {
			lines[at] = srcLine{l.n, substitute(l.text, counter, k)}
			at++
		}
	}
	return lines
}

// substitute replaces the FOR counter in the code part of text with k,
// written as two digits when joined to the name before it with &, as in
// "x&i", and as a plain number otherwise.
func substitute(text, counter string, k int) string {
	code, comment, hasComment := strings.Cut(text, ";")
	if counter == "" || !strings.Contains(code, counter) {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(code); {
		c := code[i]
		if !isIdentChar(c) || c >= '0' && c <= '9' {
			if c == '&' && strings.HasPrefix(code[i+1:], counter) && !identAt(code, i+1+len(counter)) {
				fmt.Fprintf(&b, "%02d", k)
				i += 1 + len(counter)
				continue
			}
			b.WriteByte(c)
			i++
			if c >= '0' && c <= '9' {
				for i < len(code) && isIdentChar(code[i]) {
					b.WriteByte(code[i])
					i++
				}
			}
			continue
		}
		j := i
		for j < len(code) && isIdentChar(code[j]) {
			j++
		}
		if code[i:j] == counter && (i == 0 || code[i-1] != '.') {
			fmt.Fprintf(&b, "%d", k)
		} else {
			b.WriteString(code[i:j])
		}
		i = j
	}
	if hasComment {
		b.WriteByte(';')
		b.WriteString(comment)
	}
	return b.String()
}

// constants are the predefined symbols, in upper case.
var constants = []string{"CORESIZE", "MAXPROCESSES", "MAXCYCLES", "MAXLENGTH", "MINDISTANCE", "PSPACESIZE", "WARRIORS", "VERSION", "CURLINE"}

// constant returns s in upper case if it names a predefined constant, and
// "" if not, without allocating.
func constant(s string) string {
	for _, c := range constants {
		if strings.EqualFold(s, c) {
			return c
		}
	}
	return ""
}

func isTwoCharOp(s string) bool {
	switch s {
	case "==", "!=", "<=", ">=", "&&", "||":
		return true
	}
	return false
}

func identAt(s string, i int) bool { return i < len(s) && isIdentChar(s[i]) }

func isIdentChar(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// expandEQUs replaces each EQU in an instruction's operands whose value
// starts with an addressing mode, such as "X EQU #2", or spans both
// operands, such as "X EQU 2,0", with its value. EQU is textual in pMARS,
// so such values may lead an operand or supply two. Other EQUs are left for
// eval, which treats them as parenthesised expressions.
func (a *assembler) expandEQUs(toks []string, line int) ([]string, error) {
	if len(a.equs) == 0 {
		return toks, nil
	}
	for range 32 {
		if !slices.ContainsFunc(toks[1:], a.isTextEQU) {
			return toks, nil
		}
		out := make([]string, 0, len(toks)+8)
		changed := false
		for i, t := range toks {
			if i == 0 || toks[i-1] == "." || !a.isTextEQU(t) {
				out = append(out, t)
				continue
			}
			changed = true
			body := a.equs[t]
			if slices.Contains(body, ",") {
				out = append(out, body...)
				continue
			}
			out = append(out, body[0], "(")
			out = append(out, body[1:]...)
			out = append(out, ")")
		}
		if !changed {
			return out, nil
		}
		toks = out
	}
	return nil, &Error{Line: line, Msg: "EQU is recursive"}
}

// isTextEQU reports whether t is an EQU whose value starts with a mode
// or contains a comma, and so must be substituted as text.
func (a *assembler) isTextEQU(t string) bool {
	body, ok := a.equs[t]
	return ok && len(body) > 0 && (isModeChar(body[0]) || slices.Contains(body, ","))
}

func isModeChar(t string) bool {
	return len(t) == 1 && strings.IndexByte(mars.ModeChars, t[0]) >= 0
}

func startsSpace(s string) bool { return s == "" || s[0] == ' ' || s[0] == '\t' }

func (a *assembler) define(l string, line int) error {
	if _, ok := a.labels[l]; ok {
		return &Error{Line: line, Msg: fmt.Sprintf("label %q redefined", l)}
	}
	if _, ok := a.equs[l]; ok {
		return &Error{Line: line, Msg: fmt.Sprintf("label %q redefined", l)}
	}
	a.lines[l] = line
	return nil
}

func (a *assembler) fold(v int) uint16 {
	v %= a.cfg.CoreSize
	if v < 0 {
		v += a.cfg.CoreSize
	}
	return uint16(v)
}

func parseStatement(toks []string, line int) (statement, error) {
	st := statement{line: line}
	op, ok := lookupOpcode(toks[0])
	if !ok {
		return st, &Error{Line: line, Msg: fmt.Sprintf("unknown opcode %q", toks[0])}
	}
	st.op = op
	toks = toks[1:]
	if len(toks) > 0 && toks[0] == "." {
		if len(toks) < 2 {
			return st, &Error{Line: line, Msg: "missing modifier"}
		}
		mod, ok := lookupModifier(toks[1])
		if !ok {
			return st, &Error{Line: line, Msg: fmt.Sprintf("unknown modifier %q", toks[1])}
		}
		st.mod, st.hasMod = mod, true
		toks = toks[2:]
	}
	for len(toks) > 0 {
		depth, end := 0, len(toks)
		for i, t := range toks {
			switch t {
			case "(":
				depth++
			case ")":
				depth--
			case ",":
				if depth == 0 && end == len(toks) {
					end = i
				}
			}
		}
		op := operand{mode: mars.Direct, expr: toks[:end]}
		if len(op.expr) > 0 && len(op.expr[0]) == 1 {
			if i := strings.IndexByte(mars.ModeChars, op.expr[0][0]); i >= 0 {
				op.mode, op.expr = mars.Mode(i), op.expr[1:]
			}
		}
		if len(op.expr) == 0 {
			return st, &Error{Line: line, Msg: "missing operand"}
		}
		if st.nops == len(st.ops) {
			return st, &Error{Line: line, Msg: "too many operands"}
		}
		st.ops[st.nops] = op
		st.nops++
		if end == len(toks) {
			break
		}
		toks = toks[end+1:]
		if len(toks) == 0 {
			return st, &Error{Line: line, Msg: "missing operand after ','"}
		}
	}
	return st, nil
}

func lookupOpcode(s string) (mars.Opcode, bool) {
	if strings.EqualFold(s, "CMP") {
		return mars.SEQ, true
	}
	for i := range mars.NumOpcodes {
		if strings.EqualFold(mars.Opcode(i).String(), s) {
			return mars.Opcode(i), true
		}
	}
	return 0, false
}

func lookupModifier(s string) (mars.Modifier, bool) {
	for i := range mars.NumModifiers {
		if strings.EqualFold(mars.Modifier(i).String(), s) {
			return mars.Modifier(i), true
		}
	}
	return 0, false
}

// directives are the pseudo-opcodes, in upper case.
var directives = []string{"EQU", "ORG", "END", "FOR", "ROF", "PIN"}

// directive returns s in upper case if it is a directive, and "" if not,
// without allocating.
func directive(s string) string {
	for _, d := range directives {
		if strings.EqualFold(s, d) {
			return d
		}
	}
	return ""
}

func isKeyword(s string) bool {
	if directive(s) != "" {
		return true
	}
	_, ok := lookupOpcode(s)
	return ok
}

func isIdent(s string) bool {
	c := s[0]
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// zeroExpr is the implied A operand of a one-operand DAT.
var zeroExpr = []string{"0"}

// tokenize splits a line into tokens in a buffer reused across lines, and
// returns an exactly sized copy, as statements and EQUs keep their tokens
// until assembly ends.
func (a *assembler) tokenize(s string) ([]string, error) {
	toks, err := tokenizeInto(a.tokbuf[:0], s)
	a.tokbuf = toks[:0]
	if err != nil || len(toks) == 0 {
		return nil, err
	}
	return slices.Clone(toks), nil
}

func tokenize(s string) ([]string, error) { return tokenizeInto(nil, s) }

func tokenizeInto(toks []string, s string) ([]string, error) {
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		case i+1 < len(s) && isTwoCharOp(s[i:i+2]):
			toks = append(toks, s[i:i+2])
			i += 2
		case strings.IndexByte("#$*@{<}>.,:+-/%()!", c) >= 0:
			toks = append(toks, s[i:i+1])
			i++
		default:
			return nil, fmt.Errorf("unexpected character %q", c)
		}
	}
	return toks, nil
}

// eval evaluates an expression for the instruction at address cur. Labels
// evaluate relative to cur.
func (a *assembler) eval(toks []string, cur, line int) (int, error) {
	p := parser{a: a, toks: toks, cur: cur, line: line}
	v, err := p.or()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.toks) {
		return 0, &Error{Line: line, Msg: fmt.Sprintf("unexpected %q in expression", p.toks[p.pos])}
	}
	return v, nil
}

type parser struct {
	a     *assembler
	toks  []string
	pos   int
	cur   int
	line  int
	depth int
}

func (p *parser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

// or, and, equality and relational give the comparison and logical
// operators, binding more loosely than arithmetic. They
// yield 1 for true and 0 for false.
func (p *parser) or() (int, error) { return p.binary(0) }

// levels lists the binary operators from loosest to tightest binding.
var levels = [][]string{
	{"||"},
	{"&&"},
	{"==", "!="},
	{"<", ">", "<=", ">="},
}

func (p *parser) binary(level int) (int, error) {
	next := func() (int, error) {
		if level+1 < len(levels) {
			return p.binary(level + 1)
		}
		return p.expr()
	}
	v, err := next()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if !slices.Contains(levels[level], op) {
			return v, nil
		}
		p.pos++
		r, err := next()
		if err != nil {
			return 0, err
		}
		switch op {
		case "||":
			v = b2i(v != 0 || r != 0)
		case "&&":
			v = b2i(v != 0 && r != 0)
		case "==":
			v = b2i(v == r)
		case "!=":
			v = b2i(v != r)
		case "<":
			v = b2i(v < r)
		case ">":
			v = b2i(v > r)
		case "<=":
			v = b2i(v <= r)
		case ">=":
			v = b2i(v >= r)
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (p *parser) expr() (int, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case "+":
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v += r
		case "-":
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v -= r
		default:
			return v, nil
		}
	}
}

func (p *parser) term() (int, error) {
	v, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != "*" && op != "/" && op != "%" {
			return v, nil
		}
		p.pos++
		r, err := p.unary()
		if err != nil {
			return 0, err
		}
		switch op {
		case "*":
			v *= r
		case "/", "%":
			if r == 0 {
				return 0, &Error{Line: p.line, Msg: "division by zero"}
			}
			if op == "/" {
				v /= r
			} else {
				v %= r
			}
		}
	}
}

func (p *parser) unary() (int, error) {
	switch p.peek() {
	case "-":
		p.pos++
		v, err := p.unary()
		return -v, err
	case "+":
		p.pos++
		return p.unary()
	case "!":
		p.pos++
		v, err := p.unary()
		return b2i(v == 0), err
	}
	return p.primary()
}

func (p *parser) primary() (int, error) {
	t := p.peek()
	if t == "" {
		return 0, &Error{Line: p.line, Msg: "unexpected end of expression"}
	}
	p.pos++
	switch {
	case t == "(":
		v, err := p.or()
		if err != nil {
			return 0, err
		}
		if p.peek() != ")" {
			return 0, &Error{Line: p.line, Msg: "missing ')'"}
		}
		p.pos++
		return v, nil
	case t[0] >= '0' && t[0] <= '9':
		return strconv.Atoi(t)
	case isIdent(t):
		return p.ident(t)
	}
	return 0, &Error{Line: p.line, Msg: fmt.Sprintf("unexpected %q in expression", t)}
}

func (p *parser) ident(t string) (int, error) {
	if addr, ok := p.a.labels[t]; ok {
		return addr - p.cur, nil
	}
	if toks, ok := p.a.equs[t]; ok {
		if p.depth > 32 {
			return 0, &Error{Line: p.line, Msg: fmt.Sprintf("EQU %q is recursive", t)}
		}
		sub := parser{a: p.a, toks: toks, cur: p.cur, line: p.line, depth: p.depth + 1}
		v, err := sub.or()
		if err != nil {
			return 0, err
		}
		if sub.pos != len(sub.toks) {
			return 0, &Error{Line: p.a.lines[t], Msg: fmt.Sprintf("unexpected %q in EQU %q", sub.toks[sub.pos], t)}
		}
		return v, nil
	}
	c := p.a.cfg
	switch constant(t) {
	case "CORESIZE":
		return c.CoreSize, nil
	case "MAXPROCESSES":
		return c.MaxProcesses, nil
	case "MAXCYCLES":
		return c.MaxCycles, nil
	case "MAXLENGTH":
		return c.MaxLength, nil
	case "MINDISTANCE":
		return c.MinDistance, nil
	case "PSPACESIZE":
		return c.PSpaceSize, nil
	case "WARRIORS":
		return 2, nil
	case "VERSION":
		return Version, nil
	case "CURLINE":
		return p.cur, nil
	}
	return 0, &Error{Line: p.line, Msg: fmt.Sprintf("undefined symbol %q", t)}
}
