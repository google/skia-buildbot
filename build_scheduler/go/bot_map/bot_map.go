package bot_map

/*
	This file contains mappings between swarming trigger bots and builder names.
*/

type Mapping struct {
	SwarmingBots []string
	BuilderNames []string
}

var (
	// BOT_MAP is the mapping of a set of swarming trigger bots to a set of builder names.
	BOT_MAP = []Mapping{
		Mapping{
			SwarmingBots: []string{
				"skia-trigger-117",
			},
			BuilderNames: []string{
				"Build-Ubuntu-GCC-x86_64-Release-Swarmbucket",
			},
		},
	}

	// BUILDERS_BY_SWARMING_BOT is a view of the BOT_MAP which maps individual swarming
	// trigger bots to the set of builders which they may run.
	BUILDERS_BY_SWARMING_BOT = map[string][]string{}

	// SWARMING_BOTS_BY_BUILDER is a view of the BOT_MAP which maps individual builder
	// names to the set of swarming trigger bots which may run them.
	SWARMING_BOTS_BY_BUILDER = map[string][]string{}
)

func init() {
	for _, mapping := range BOT_MAP {
		for _, swarmingBot := range mapping.SwarmingBots {
			BUILDERS_BY_SWARMING_BOT[swarmingBot] = make([]string, len(mapping.BuilderNames))
			copy(BUILDERS_BY_SWARMING_BOT[swarmingBot], mapping.BuilderNames)
		}
		for _, builderName := range mapping.BuilderNames {
			SWARMING_BOTS_BY_BUILDER[builderName] = make([]string, len(mapping.SwarmingBots))
			copy(SWARMING_BOTS_BY_BUILDER[builderName], mapping.SwarmingBots)
		}
	}
}
