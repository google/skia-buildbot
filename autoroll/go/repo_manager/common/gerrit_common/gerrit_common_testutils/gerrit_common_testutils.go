package gerrit_common_testutils

import (
	"go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
)

// TODO(borenet): Probably don't need this 4-line package.
func SetupMockGerrit(t sktest.TestingT, urlmock *mockhttpclient.URLMock) *testutils.MockGerrit {
	mockGerrit := testutils.NewGerrit(t, urlmock)
	mockGerrit.MockGetUserEmail()
	return mockGerrit
}
