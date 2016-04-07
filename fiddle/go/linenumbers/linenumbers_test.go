package linenumbers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineNumbers(t *testing.T) {
	code := `a
b
c`
	want := `#line 1
a
#line 2
b
#line 3
c`
	assert.Equal(t, want, LineNumbers(code))

	code = ``
	want = `#line 1
`
	assert.Equal(t, want, LineNumbers(code))
}
