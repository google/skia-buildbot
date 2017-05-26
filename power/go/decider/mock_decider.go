package decider

import (
	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/mock"
)

type MockDecider struct {
	mock.Mock
}

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
