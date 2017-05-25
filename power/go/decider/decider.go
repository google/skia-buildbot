package decider

import swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"

// The BotDecider interface abstracts away a configuration file or similar which indicates which bots have powercyclable devices attached and which are golo machines, etc
type Decider interface {
	ShouldPowercycleBot(*swarming.SwarmingRpcsBotInfo) bool
	ShouldPowercycleDevice(*swarming.SwarmingRpcsBotInfo) bool
	GetBugURL(*swarming.SwarmingRpcsBotInfo) string
}
