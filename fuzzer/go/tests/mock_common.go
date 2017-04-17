package tests

import (
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/fuzzer/go/common"
)

// MockCommonImpl is a mock of common.CommonImpl. All the methods are mocked using testify's mocking
// library.
type MockCommonImpl struct {
	mock.Mock
}

// New returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockCommonImpl() *MockCommonImpl {
	return &MockCommonImpl{}
}

func (m *MockCommonImpl) Hostname() string {
	args := m.Called()
	return args.String(0)
}

// Make sure MockCommonImpl fulfills common.CommonImpl
var _ common.CommonImpl = (*MockCommonImpl)(nil)
