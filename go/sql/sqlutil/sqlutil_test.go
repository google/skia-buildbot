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

func TestWherePlaceholders_ValdInputs_Success(t *testing.T) {
	w := WherePlaceholders([]string{"col1"}, 1)
	assert.Equal(t, "(col1=$1)", w)

	w = WherePlaceholders([]string{"col1"}, 2)
	assert.Equal(t, "(col1=$1) OR (col1=$2)", w)

	w = WherePlaceholders([]string{"col1", "col2"}, 1)
	assert.Equal(t, "(col1=$1 AND col2=$2)", w)

	w = WherePlaceholders([]string{"col1", "col2"}, 2)
	assert.Equal(t, "(col1=$1 AND col2=$2) OR (col1=$3 AND col2=$4)", w)
}

func TestWherePlaceholders_InvalidInputs_Panics(t *testing.T) {
	assert.Panics(t, func() {
		WherePlaceholders([]string{}, 2)
	})
	assert.Panics(t, func() {
		WherePlaceholders([]string{"col1"}, 0)
	})
	assert.Panics(t, func() {
		WherePlaceholders([]string{"col1"}, -2)
	})
}
