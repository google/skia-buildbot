package calc

import (
	"testing"
)

func TestLex(t *testing.T) {
	testCases := []struct {
		input string
		items []item
	}{
		{
			input: "foo()",
			items: []item{
				{itemIdentifier, "foo"},
				{itemLParen, "("},
				{itemRParen, ")"},
				{itemEOF, ""},
			},
		},
		{
			input: "foo(a, b) ",
			items: []item{
				{itemIdentifier, "foo"},
				{itemLParen, "("},
				{itemIdentifier, "a"},
				{itemComma, ","},
				{itemIdentifier, "b"},
				{itemRParen, ")"},
				{itemEOF, ""},
			},
		},
		{
			input: " foo( \"stuff goes here\")",
			items: []item{
				{itemIdentifier, "foo"},
				{itemLParen, "("},
				{itemString, "stuff goes here"},
				{itemRParen, ")"},
				{itemEOF, ""},
			},
		},
		{
			input: " foo(bar(\"stuff goes here\", 1e-9,  baz()))",
			items: []item{
				{itemIdentifier, "foo"},
				{itemLParen, "("},
				{itemIdentifier, "bar"},
				{itemLParen, "("},
				{itemString, "stuff goes here"},
				{itemComma, ","},
				{itemNum, "1e-9"},
				{itemComma, ","},
				{itemIdentifier, "baz"},
				{itemLParen, "("},
				{itemRParen, ")"},
				{itemRParen, ")"},
				{itemRParen, ")"},
				{itemEOF, ""},
			},
		},
	}
	for _, tc := range testCases {
		l := newLexer(tc.input)
		for _, ex := range tc.items {
			it := l.nextItem()
			if got, want := it.typ, ex.typ; got != want {
				t.Fatalf("Wrong type: Got %v Want %v", got, want)
			}
			if got, want := it.val, ex.val; got != want {
				t.Fatalf("Wrong value: Got %v Want %v", got, want)
			}
		}
	}
}

func TestLexErrors(t *testing.T) {

	testCases := []string{
		"foo}",
		"{a, b ",
		" foo( \"stuff goes here)",
	}
	for _, tc := range testCases {
		l := newLexer(tc)
		it := l.nextItem()
		for ; it.typ != itemEOF && it.typ != itemError; it = l.nextItem() {
		}
		if it.typ == itemEOF {
			t.Errorf("%s should have failed to parse", tc)
		}
	}
}
