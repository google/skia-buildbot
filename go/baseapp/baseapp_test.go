package baseapp

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSecurityMiddleware_NotLocalNoOptions(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE  'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{}))
}

func TestSecurityMiddleware_LocalNoOptions(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-eval' 'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, true, []Option{}))
}

func TestSecurityMiddleware_NotLocalAllowWASM(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-eval' 'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{AllowWASM{}}))
}

func TestSecurityMiddleware_NotLocalAllowAnyImages(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "base-uri 'none';  img-src * 'unsafe-eval' blob: data: ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE  'unsafe-inline' https: http: ; report-uri /cspreport ;", cspString([]string{"https://example.org"}, false, []Option{AllowAnyImage{}}))
}
