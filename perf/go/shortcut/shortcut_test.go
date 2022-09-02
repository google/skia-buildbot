package shortcut

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIDFromKeys(t *testing.T) {

	assert.Equal(t, "X", IDFromKeys(nil))

	assert.Equal(t, "X8a18d2be561b75dc48c3afcd8145767c", IDFromKeys(&Shortcut{
		Keys: []string{
			",arch=x86,config=8888,",
			",arch=arm,config=8888,",
		},
	}))

	// Test that order doesn't matter.
	assert.Equal(t, "X8a18d2be561b75dc48c3afcd8145767c", IDFromKeys(&Shortcut{
		Keys: []string{
			",arch=arm,config=8888,",
			",arch=x86,config=8888,",
		},
	}))
}
