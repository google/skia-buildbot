package instance_types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbeddedSetupScript_CorrectlyEmbedded(t *testing.T) {
	assert.Contains(t, embeddedSetupScript, "#!/bin/bash")
}
