package decider

import (
	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/mock"
)

type MockDecider struct {
	mock.Mock
}

// NewMockDecider returns a pointer to a newly created struct.  We return the pointer because we want to
// make sure the methods on mock.Mock stay accessible, e.g. m.On()
func NewMockDecider() *MockDecider {
	return &MockDecider{}
}

func (m *MockDecider) ShouldPowercycleBot(bot *swarming.SwarmingRpcsBotInfo) bool {
	args := m.Called(bot)
	return args.Bool(0)
}

func (m *MockDecider) ShouldPowercycleDevice(bot *swarming.SwarmingRpcsBotInfo) bool {
	args := m.Called(bot)
	return args.Bool(0)
}

// Ensure MockDecider fulfills Decider
var _ Decider = (*MockDecider)(nil)
