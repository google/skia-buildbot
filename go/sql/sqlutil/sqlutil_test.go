package sqlutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValuesPlaceholders_ValidInputs_Success(t *testing.T) {

	v := ValuesPlaceholders(3, 2)
	assert.Equal(t, "($1,$2,$3),($4,$5,$6)", v)

	v = ValuesPlaceholders(2, 4)
	assert.Equal(t, "($1,$2),($3,$4),($5,$6),($7,$8)", v)

	v = ValuesPlaceholders(1, 1)
	assert.Equal(t, "($1)", v)

	v = ValuesPlaceholders(1, 3)
	assert.Equal(t, "($1),($2),($3)", v)
}

func TestValuesPlaceholders_InvalidInputs_Panics(t *testing.T) {

	assert.Panics(t, func() {
		ValuesPlaceholders(-3, 2)
	})
	assert.Panics(t, func() {
		ValuesPlaceholders(2, -4)
	})
	assert.Panics(t, func() {
		ValuesPlaceholders(0, 0)
	})
}
