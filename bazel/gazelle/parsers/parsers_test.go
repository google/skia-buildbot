package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestSplitLinesAndRemoveComments_Success(t *testing.T) {
	unittest.SmallTest(t)

	const input = `
First // inline comments are kept
Second				// inline comments are kept
Third //comment
// but not fully commented out lines
   //even with spaces at the beginning
/* another comment */Fourth
Fifth /** another comment */
Six/** comment **/th
/*
Block comment
block
*/
Seventh`

	assert.Equal(t, []string{
		"",
		"First // inline comments are kept",
		"Second\t\t\t\t// inline comments are kept",
		"Third //comment",
		"Fourth",
		"Fifth ",
		"Sixth",
		"",
		"",
		"Seventh",
	}, SplitLinesAndRemoveComments(input))
}

func TestSplitLinesAndRemoveComments_HandlesMultipleBlockCommentsOnALine(t *testing.T) {
	unittest.SmallTest(t)

	const input = `#include "alpha.h"
/* foo */ /* bar */
#include "beta.h"
apple/* foo */=/* bar */orange
#include "gamma.h"
/* done */`

	assert.Equal(t, []string{
		`#include "alpha.h"`,
		" ",
		`#include "beta.h"`,
		"apple=orange",
		`#include "gamma.h"`,
		"",
	}, SplitLinesAndRemoveComments(input))
}
