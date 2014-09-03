package parser

import "testing"

func TestLex(t *testing.T) {

	testCases := []struct {
		input string
		items []item
	}{
		{
			input: "foo()",
			items: []item{
				item{itemIdentifier, "foo"},
				item{itemLParen, "("},
				item{itemRParen, ")"},
				item{itemEOF, ""},
			},
		},
		{
			input: "foo(a, b) ",
			items: []item{
				item{itemIdentifier, "foo"},
				item{itemLParen, "("},
				item{itemIdentifier, "a"},
				item{itemComma, ","},
				item{itemIdentifier, "b"},
				item{itemRParen, ")"},
				item{itemEOF, ""},
			},
		},
		{
			input: " foo( \"stuff goes here\")",
			items: []item{
				item{itemIdentifier, "foo"},
				item{itemLParen, "("},
				item{itemString, "stuff goes here"},
				item{itemRParen, ")"},
				item{itemEOF, ""},
			},
		},
		{
			input: " foo(bar(\"stuff goes here\", 1e-9,  baz()))",
			items: []item{
				item{itemIdentifier, "foo"},
				item{itemLParen, "("},
				item{itemIdentifier, "bar"},
				item{itemLParen, "("},
				item{itemString, "stuff goes here"},
				item{itemComma, ","},
				item{itemNum, "1e-9"},
				item{itemComma, ","},
				item{itemIdentifier, "baz"},
				item{itemLParen, "("},
				item{itemRParen, ")"},
				item{itemRParen, ")"},
				item{itemRParen, ")"},
				item{itemEOF, ""},
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
