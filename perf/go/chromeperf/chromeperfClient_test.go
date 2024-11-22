package chromeperf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenereateUrl_Bridge(t *testing.T) {
	api := "api_name"
	function := "func_name"
	direct := false
	urlOverride := ""

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, "https://skia-bridge-dot-chromeperf.appspot.com/api_name/func_name")
}

func TestGenereateUrl_Direct(t *testing.T) {
	api := "api_name"
	function := "func_name" // will be ignored
	direct := true
	urlOverride := ""

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, "https://chromeperf.appspot.com/api_name")
}

func TestGenereateUrl_Override(t *testing.T) {
	api := "api_name"       // will be ignored
	function := "func_name" // will be ignored
	direct := true          // will be ignored
	urlOverride := "override.url/someapi/andfunction"

	url := generateTargetUrl(urlOverride, direct, api, function)
	assert.Equal(t, url, urlOverride)
}
