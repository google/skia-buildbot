// Package bot_configs defines the configurations used by all
// bots in Pinpoint.
//
// When building Chrome, the configurations define which builder to use.
// When running tests, the configurations define the command to give Swarming
// and directs the device pool to use.
//
// This package also runs some rudimentary validation tests. You can self
// validate with:
//
//	bazelisk test //pinpoint/go/bot_configs/...
//
// The bot configurations are also defined [here]. They were copied over
// on May 6, 2025.
//
// [here]: https://chromeperf.appspot.com/edit_site_config?key=bot_configurations
package bot_configs

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

//go:embed external.json
var externalBotConfigsJSON []byte

var (
	externalBotConfigs map[string]BotConfig
	onceExternal       sync.Once
)

func getExternalBotConfigs() map[string]BotConfig {
	onceExternal.Do(func() {
		//workflowcheck:ignore
		err := json.Unmarshal(externalBotConfigsJSON, &externalBotConfigs)
		if err != nil {
			externalBotConfigs = make(map[string]BotConfig)
			sklog.Errorf("Fail to load external bot config file: %s", err)
		}
	})
	return externalBotConfigs
}

//go:embed internal.json
var internalBotConfigsJSON []byte

var (
	allBotConfigs map[string]BotConfig
	onceAll       sync.Once
)

func getAllBotConfigs() map[string]BotConfig {
	onceAll.Do(func() {
		allBotConfigs = maps.Clone(getExternalBotConfigs())
		//workflowcheck:ignore
		err := json.Unmarshal(internalBotConfigsJSON, &allBotConfigs)
		if err != nil {
			sklog.Errorf("Fail to load internal bot config file: %s", err)
		}
	})
	return allBotConfigs
}

// A BotConfig contains the parameters that make up
// the configuration. Configurations should either
// include an alias or all other fields.
type BotConfig struct {
	// Alias defines another bot that uses the same
	// configuration as this one.
	Alias string `json:"alias"`
	// Browser is the name of the Chrome browser to run
	// when testing benchmarks
	Browser string `json:"browser"`
	// Bucket refers to the LUCI pool to build Chrome,
	// typically luci.chrome.try
	Bucket string `json:"bucket"`
	// Builder refers to the LUCI builder used to build Chrome,
	// usually a compile builder.
	Builder string `json:"builder"`
	// Repo is the repository to run tests on,
	// typically chromium
	Repo string `json:"repository"`
	// SwarmingServer is almost always
	// https://chrome-swarming.appspot.com
	SwarmingServer string `json:"swarming_server"`
	// Bot is the original key used for this config.
	Bot string
	// Dimensions are used by Swarming to find the
	// device pool to test benchmarks
	Dimensions []map[string]string `json:"dimensions"`
}

// GetBotConfig gets the config for a bot. Will only check internal
// data if externalOnly = false.
func GetBotConfig(bot string, externalOnly bool) (BotConfig, error) {
	var botConfigs map[string]BotConfig

	if externalOnly {
		botConfigs = getExternalBotConfigs()
	} else {
		botConfigs = getAllBotConfigs()
	}

	cfg, ok := botConfigs[bot]
	if !ok {
		var errMsg string
		if externalOnly {
			errMsg = fmt.Sprintf("bot %s was not found in external only bot configuration data", bot)
		} else {
			errMsg = fmt.Sprintf("bot %s was not found in internal and external bot configuration data", bot)
		}
		return BotConfig{}, skerr.Fmt("%s", errMsg)
	}
	alias := cfg.Alias
	if alias != "" {
		cfg = botConfigs[cfg.Alias]
		cfg.Bot = alias
		return cfg, nil
	}

	cfg.Bot = bot
	return cfg, nil
}

// validate ensures the bot configuration files are correct
// Those rules are roughly:
// - any configuration is defined in either the external or internal
// data file, not both
// - aliases point to another bot
// - if alias is defined in external config, it does not point to a bot
// in the internal config
// - aliases do not refer to another device that also uses an alias
// - if alias is defined, the no other fields should be defined
// - if alias is not defined, then all other fields should be defined
func validate() error {
	var external map[string]*BotConfig
	var internal map[string]*BotConfig
	var botConfigs map[string]*BotConfig

	err := json.Unmarshal(externalBotConfigsJSON, &external)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = json.Unmarshal(internalBotConfigsJSON, &internal)
	if err != nil {
		return skerr.Wrap(err)
	}
	// verify any internal bots are also defined in external
	for name := range internal {
		_, ok := external[name]
		if ok {
			return skerr.Fmt("%s bot is defined in both internal and external configurations", name)
		}
	}

	// validate aliases in external config
	for name, bot := range external {
		alias := bot.Alias
		if alias != "" {
			nextBot, ok := external[alias]
			if !ok {
				return skerr.Fmt("%s uses alias %s that is not defined in the external configs", name, alias)
			}
			if nextBot.Alias != "" {
				return skerr.Fmt("%s cannot have nested aliases in bot configurations", name)
			}
		}
	}

	// combine maps and validate each device
	err = json.Unmarshal(externalBotConfigsJSON, &botConfigs)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = json.Unmarshal(internalBotConfigsJSON, &botConfigs)
	if err != nil {
		return skerr.Wrap(err)
	}
	for name, bot := range botConfigs {
		alias := bot.Alias
		if alias != "" {
			nextBot, ok := botConfigs[alias]
			if !ok {
				return skerr.Fmt("%s uses alias %s that is not defined in either config", name, alias)
			}
			if nextBot.Alias != "" {
				return skerr.Fmt("%s cannot have nested aliases in bot configurations", name)
			}
			if bot.Browser != "" || bot.Builder != "" || len(bot.Dimensions) > 0 ||
				bot.Repo != "" || bot.SwarmingServer != "" {
				return skerr.Fmt("%s defines both an alias and other fields. Do one or the other.", name)
			}
			continue
		}

		// verify bot has all other parameters defined
		switch {
		case bot.Browser == "":
			return skerr.Fmt("%s is missing browser configs", name)
		case bot.Builder == "":
			return skerr.Fmt("%s is missing builder configs", name)
		case len(bot.Dimensions) == 0:
			return skerr.Fmt("%s does not have any dimensions", name)
		case bot.Repo == "":
			return skerr.Fmt("%s is missing repository configs", name)
		case bot.SwarmingServer == "":
			return skerr.Fmt("%s is missing swarming server configs", name)
		}
	}

	return nil
}
