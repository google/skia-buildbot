package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlertConfigFromString(t *testing.T) {
	value, err := AlertConfigStateFromName("ACTIVE")
	assert.NoError(t, err)
	assert.Equal(t, ACTIVE, value)

	value, err = AlertConfigStateFromName("aCTIVE")
	assert.Error(t, err)

	value, err = AlertConfigStateFromName("other")
	assert.Error(t, err)

	value, err = AlertConfigStateFromName("EOL")
	assert.Error(t, err)
}
