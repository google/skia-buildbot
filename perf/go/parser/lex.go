package parser

import (
	"fmt"
	"strings"
	"unicode"
)

type itemType int

const (
	itemError itemType = iota
	itemIdentifier
	itemNum
	itemString
	itemLParen
	itemRParen
	itemComma
	itemEOF
)

const eof byte = 0xff

// item is returned by the lexer.nextItem() as it parses in the input.
type item struct {
	typ itemType
	val string
}

// stateFn is a function that represents the current state of the lexer.
type stateFn func(*lexer) stateFn

// lexer parses an input string and returns items for each lexeme that's found.
type lexer struct {
	input      string    // The string being parsed.
	start      int       // The offset of the current lexical item.
	pos        int       // Current position in input.
	items      chan item // Channel by which items are delivered.
	state      stateFn   // The next lexing function.
	peekBuffer []item    // A peekBuffer for peek'd items.
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	if len(l.peekBuffer) > 0 {
		item := l.peekBuffer[0]
		l.peekBuffer = l.peekBuffer[:0]
		return item
	}
	item := <-l.items
	return item
}

// peekItem allows the caller to look ahead and see the next item that
// nextItem() will return.
func (l *lexer) peekItem() item {
	item := <-l.items
	l.peekBuffer = append(l.peekBuffer, item)
	return item
}

// accept consumes the next char if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexByte(valid, l.next()) >= 0 {
		return true
	}
	l.backUp()
	return false
}

// acceptRun consumes a run of chars from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.IndexByte(valid, l.next()) >= 0 {
	}
	l.backUp()
}

// ingore the current text.
func (l *lexer) ignore() {
	l.start = l.pos
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.run.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{typ: itemError, val: fmt.Sprintf(format, args...)}
	return nil
}

// newLexer returns a new lexer for the given string.
func newLexer(input string) *lexer {
	l := &lexer{
		input:      input,
		start:      0,
		pos:        0,
		items:      make(chan item, 2),
		state:      lexExp,
		peekBuffer: []item{},
	}
	go l.run()
	return l
}

// next returns the next char in the input.
func (l *lexer) next() byte {
	if int(l.pos) >= len(l.input) {
		return eof
	}
	ch := l.input[l.pos]
	l.pos += 1
	return ch
}

// backUp steps back one rune. Can only be called once per call of next.
func (l *lexer) backUp() {
	l.pos -= 1
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.state = lexExp; l.state != nil; l.state = l.state(l) {
	}
}

// emit puts a new item on the channel.
func (l *lexer) emit(t itemType) {
	l.items <- item{
		typ: t,
		val: l.input[l.start:l.pos],
	}
	l.start = l.pos
}

// lexExp parses the input expression.
func lexExp(l *lexer) stateFn {
	switch r := l.next(); {
	case r == eof:
		l.emit(itemEOF)
		return nil
	case r == '"':
		return lexString
	case unicode.IsLetter(rune(r)):
		return lexIdentifier
	case r == ')':
		l.emit(itemRParen)
		return lexExp
	case r == '(':
		l.emit(itemLParen)
		return lexExp
	case r == ',':
		l.emit(itemComma)
		return lexExp
	case unicode.IsSpace(rune(r)):
		l.ignore()
		return lexExp
	case r == '+' || r == '-' || ('0' <= r && r <= '9'):
		l.backUp()
		return lexNumber
	default:
		return l.errorf("unrecognized char: %#U", r)
	}
}

// lexString parses double-quote delimited strings.
func lexString(l *lexer) stateFn {
	l.ignore()
	r := l.next()
	for ; r != eof; r = l.next() {
		if r == '"' {
			l.backUp()
			l.emit(itemString)
			l.next()
			l.ignore()
			break
		}
	}
	if r == eof {
		l.errorf("Unterminated string: %s", l.input[l.start:l.pos])
	}
	return lexExp
}

// lexNumber parses numbers, things that looks like ints and floats.
func lexNumber(l *lexer) stateFn {
	// Optional leading sign.
	l.accept("+-")
	// Is it hex?
	digits := "0123456789"
	if l.accept("0") && l.accept("xX") {
		digits = "0123456789abcdefABCDEF"
	}
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789")
	}
	l.emit(itemNum)
	return lexExp
}

// lexIdentifier parses function names.
func lexIdentifier(l *lexer) stateFn {
	for {
		r := l.next()
		if !unicode.IsLetter(rune(r)) && !unicode.IsDigit(rune(r)) {
			l.backUp()
			break
		}
	}
	l.emit(itemIdentifier)
	return lexExp
}
