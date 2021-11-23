package parsers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseCIncludes_Success(t *testing.T) {
	unittest.SmallTest(t)

	const source = `/* Sample C++ file with imports. */
#include "include/private/SkMacros.h"
	#include "include/private/SkDeque.h"

#include   <vector>

#include	<cstring>
#include  <memory> // This comment should be ignored.
 #include <string.h>

#include "png.h"
#include "dawn/webgpu_cpp.h"

// Even though these are #if guarded, we should still list them.
#if SK_SUPPORT_GPU
#include "include/gpu/GrDirectContext.h"
#include "src/gpu/BaseDevice.h"
#include "src/gpu/SkGr.h"
#if defined(SK_BUILD_FOR_ANDROID_FRAMEWORK)
#   include "src/gpu/GrRenderTarget.h"
#   include "src/gpu/GrRenderTargetProxy.h"
#endif
#endif

// Line comments should be ignored.
//
// #include "experimental/foo.h"

// Block comments should be ignored.
/*
#include "experimental/bar.h"
*/

// It is ok to include cpp files and files with strange capitalization
#include "src/core/alpha.cpp"
#include "src/core/beta.C"
#include "src/core/gamma.H"

// Files which are not C/C++ headers or sources should be ignored
#include "src/core/foo.sksl"
`

	expectedRepoIncludes := []string{
		"dawn/webgpu_cpp.h",
		"include/gpu/GrDirectContext.h",
		"include/private/SkDeque.h",
		"include/private/SkMacros.h",
		"png.h",
		"src/core/alpha.cpp",
		"src/core/beta.C",
		"src/core/gamma.H",
		"src/gpu/BaseDevice.h",
		"src/gpu/GrRenderTarget.h",
		"src/gpu/GrRenderTargetProxy.h",
		"src/gpu/SkGr.h",
	}

	expectedSystemIncludes := []string{
		"cstring",
		"memory",
		"string.h",
		"vector",
	}

	actualRepoIncludes, actualSystemIncludes := ParseCIncludes(source)
	require.Equal(t, expectedRepoIncludes, actualRepoIncludes)
	require.Equal(t, expectedSystemIncludes, actualSystemIncludes)
}
