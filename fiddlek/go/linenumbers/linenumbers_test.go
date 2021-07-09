package linenumbers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestLineNumbers(t *testing.T) {
	unittest.SmallTest(t)
	code := `a
b
c`
	want := `#line 1
a
b
c`
	assert.Equal(t, want, LineNumbers(code))

	code = ``
	want = `#line 1
`
	assert.Equal(t, want, LineNumbers(code))
}
