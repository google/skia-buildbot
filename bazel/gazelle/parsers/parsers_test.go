package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLinesAndRemoveComments_Success(t *testing.T) {

	const input = `
First // inline comments are kept
Second				// inline comments are kept
Third //comment
// but not fully commented out lines
   //even with spaces at the beginning
/* another comment */Fourth
Fifth /** another comment */
Six/** comment **/th
Seventh - a block comment begins in this line /*
Block comment
block
*/ Eighth - a block comment ends in this line
Ninth`

	verbatim, noComments := SplitLinesAndRemoveComments(input)

	// Length should always be the same for both.
	assert.Len(t, verbatim, 14)
	assert.Len(t, noComments, 14)

	assert.Equal(t, []string{
		"",
		"First // inline comments are kept",
		"Second				// inline comments are kept",
		"Third //comment",
		"// but not fully commented out lines",
		"   //even with spaces at the beginning",
		"/* another comment */Fourth",
		"Fifth /** another comment */",
		"Six/** comment **/th",
		"Seventh - a block comment begins in this line /*",
		"Block comment",
		"block",
		"*/ Eighth - a block comment ends in this line",
		"Ninth",
	}, verbatim)

	assert.Equal(t, []string{
		"",
		"First // inline comments are kept",
		"Second				// inline comments are kept",
		"Third //comment",
		"",
		"",
		"Fourth",
		"Fifth ",
		"Sixth",
		"Seventh - a block comment begins in this line ",
		"",
		"",
		" Eighth - a block comment ends in this line",
		"Ninth",
	}, noComments)
}

func TestSplitLinesAndRemoveComments_HandlesMultipleBlockCommentsOnALine(t *testing.T) {

	const input = `#include "alpha.h"
/* foo */ /* bar */
#include "beta.h"
apple/* foo */=/* bar */orange
#include "gamma.h"
/* done */`

	verbatim, noComments := SplitLinesAndRemoveComments(input)

	// Length should always be the same for both.
	assert.Len(t, verbatim, 6)
	assert.Len(t, noComments, 6)

	assert.Equal(t, []string{
		`#include "alpha.h"`,
		"/* foo */ /* bar */",
		`#include "beta.h"`,
		"apple/* foo */=/* bar */orange",
		`#include "gamma.h"`,
		"/* done */",
	}, verbatim)

	assert.Equal(t, []string{
		`#include "alpha.h"`,
		" ",
		`#include "beta.h"`,
		"apple=orange",
		`#include "gamma.h"`,
		"",
	}, noComments)
}
