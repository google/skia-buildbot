package parser

import (
	"fmt"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/perf/go/types"
)

type NodeType int

const (
	NodeError NodeType = iota
	NodeFunc
	NodeNum
	NodeString
)

// Node is a single node in the parse tree.
type Node struct {
	Typ  NodeType
	Val  string
	Args []*Node
}

// newNode creates a new Node of the given type and value.
func newNode(val string, typ NodeType) *Node {
	return &Node{
		Typ:  typ,
		Val:  val,
		Args: []*Node{},
	}
}

// Evaluates a node. Only valid to call on Nodes of type NodeFunc.
func (n *Node) Eval(ctx *Context) ([]*types.PerfTrace, error) {
	if n.Typ != NodeFunc {
		return nil, fmt.Errorf("Tried to call eval on a non-Func node: %s", n.Val)
	}
	if f, ok := ctx.Funcs[n.Val]; ok {
		return f.Eval(ctx, n)
	} else {
		return nil, fmt.Errorf("Unknown function name: %s", n.Val)
	}
}

// Func defines a type for functions that can be used in the parser.
//
// The traces returned will always have a Param of "id" that identifies
// the trace. See DESIGN.md for the Trace ID naming conventions.
type Func interface {
	Eval(*Context, *Node) ([]*types.PerfTrace, error)
	Describe() string
}

// Context stores all the info for a single parser.
//
// A Context is not safe to call from multiple go routines.
type Context struct {
	Tile    *tiling.Tile
	Funcs   map[string]Func
	formula string // The current formula being evaluated.
}

// NewContext create a new parsing context that includes the basic functions.
func NewContext(tile *tiling.Tile) *Context {
	return &Context{
		Tile: tile,
		Funcs: map[string]Func{
			"filter": filterFunc,
			"norm":   normFunc,
			"fill":   fillFunc,
			"ave":    aveFunc,
			"avg":    aveFunc,
			"count":  countFunc,
			"ratio":  ratioFunc,
			"sum":    sumFunc,
			"geo":    geoFunc,
			"log":    logFunc,
		},
	}
}

// Eval parses and evaluates the given string expression and returns the Traces, or
// an error.
func (ctx *Context) Eval(exp string) ([]*types.PerfTrace, error) {
	ctx.formula = exp
	n, err := parse(exp)
	if err != nil {
		return nil, fmt.Errorf("Eval: failed to parse the expression: %s", err)
	}
	traces, err := n.Eval(ctx)
	if err == nil {
		for _, tr := range traces {
			tr.Params()["formula"] = exp
		}
	}
	return traces, err
}

// parse starts the parsing.
func parse(input string) (*Node, error) {
	l := newLexer(input)
	return parseExp(l)
}

// parseExp parses an expression.
//
// Something of the form:
//
//    fn(arg1, args2)
//
func parseExp(l *lexer) (*Node, error) {
	it := l.nextItem()
	if it.typ != itemIdentifier {
		return nil, fmt.Errorf("Expression: must begin with an identifier")
	}
	n := newNode(it.val, NodeFunc)
	it = l.nextItem()
	if it.typ != itemLParen {
		return nil, fmt.Errorf("Expression: didn't find '(' after an identifier.")
	}
	if err := parseArgs(l, n); err != nil {
		return nil, fmt.Errorf("Expression: failed parsing arguments: %s", err)
	}
	it = l.nextItem()
	if it.typ != itemRParen {
		return nil, fmt.Errorf("Expression: didn't find ')' after arguments.")
	}
	return n, nil
}

// parseArgs parses the arguments to a function.
//
// Something of the form:
//
//    arg1, arg2, arg3
//
// It terminates when it sees a closing paren, or an invalid token.
func parseArgs(l *lexer, p *Node) error {
Loop:
	for {
		it := l.peekItem()
		switch it.typ {
		case itemIdentifier:
			next, err := parseExp(l)
			if err != nil {
				return fmt.Errorf("Failed parsing args: %s", err)
			}
			p.Args = append(p.Args, next)
		case itemString:
			l.nextItem()
			node := newNode(it.val, NodeString)
			p.Args = append(p.Args, node)
		case itemNum:
			l.nextItem()
			node := newNode(it.val, NodeNum)
			p.Args = append(p.Args, node)
		case itemComma:
			l.nextItem()
			continue
		case itemRParen:
			break Loop
		default:
			return fmt.Errorf("Invalid token in args: %d", it.typ)
		}
	}
	return nil
}
