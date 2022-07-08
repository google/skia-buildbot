package standalone

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsOfAllPrecisions(t *testing.T) {
	assert.Equal(t, versionsOfAllPrecisions("12"), []string{"Mac", "Mac-12"})
	assert.Equal(t, versionsOfAllPrecisions("12.4"), []string{"Mac", "Mac-12", "Mac-12.4"})
	assert.Equal(t, versionsOfAllPrecisions("12.4.35"), []string{"Mac", "Mac-12", "Mac-12.4", "Mac-12.4.35"})
}
