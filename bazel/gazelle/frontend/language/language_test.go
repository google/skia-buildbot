package language

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAppPageDir_Success(t *testing.T) {

	assert.False(t, isAppPageDir(""))
	assert.False(t, isAppPageDir("myapp"))
	assert.False(t, isAppPageDir("myapp/util"))
	assert.False(t, isAppPageDir("myapp/modules"))
	assert.False(t, isAppPageDir("myapp/modules/my-element-sk"))
	assert.False(t, isAppPageDir("myapp/pages/static"))
	assert.True(t, isAppPageDir("myapp/pages"))
}

func TestExtractCustomElementNameFromDir_Success(t *testing.T) {

	ok, _ := extractCustomElementNameFromDir("")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp/util")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp/pages")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp/modules")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp/modules/my-element")
	assert.False(t, ok)
	ok, _ = extractCustomElementNameFromDir("myapp/modules/my-element-sk/testdata")
	assert.False(t, ok)
	ok, name := extractCustomElementNameFromDir("myapp/modules/my-element-sk")
	assert.True(t, ok)
	assert.Equal(t, "my-element-sk", name)
}
