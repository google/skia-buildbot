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
//	bazelisk test //bisection/go/bot_configs/...
//
// The bot configurations are also defined [here]. They were copied over
// on Dec 11, 2023.
//
// [here]: https://chromeperf.appspot.com/edit_site_config?key=bot_configurations
package bot_configs

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed external.json
var externalBotConfigs []byte

//go:embed internal.json
var internalBotConfigs []byte

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
	// Builder refers to the LUCI pool to build Chrome,
	// typically luci.chrome.try
	Builder string `json:"builder"`
	// Dimensions are used by Swarming to find the
	// device pool to test benchmarks
	Dimensions []map[string]string `json:"dimensions"`
	// Repo is the repository to run tests on,
	// typically chromium
	Repo string `json:"repository"`
	// SwarmingServer is almost always
	// https://chrome-swarming.appspot.com
	SwarmingServer string `json:"swarming_server"`
}

// GetBotConfig gets the config for a bot. Will only check internal
// data if externalOnly = false.
func GetBotConfig(bot string, externalOnly bool) (BotConfig, error) {
	var botConfigs map[string]BotConfig

	err := json.Unmarshal(externalBotConfigs, &botConfigs)
	if err != nil {
		return BotConfig{}, err
	}
	if !externalOnly {
		err = json.Unmarshal(internalBotConfigs, &botConfigs)
		if err != nil {
			return BotConfig{}, err
		}
	}
	cfg, ok := botConfigs[bot]
	if !ok {
		var errMsg string
		if externalOnly {
			errMsg = fmt.Sprintf("bot %s was not found in external only bot configuration data", bot)
		} else {
			errMsg = fmt.Sprintf("bot %s was not found in internal and external bot configuration data", bot)
		}
		return BotConfig{}, fmt.Errorf(errMsg)
	}
	if cfg.Alias != "" {
		return botConfigs[cfg.Alias], nil
	}
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
	var external map[string]BotConfig
	var internal map[string]BotConfig
	var botConfigs map[string]BotConfig

	err := json.Unmarshal(externalBotConfigs, &external)
	if err != nil {
		return err
	}
	err = json.Unmarshal(internalBotConfigs, &internal)
	if err != nil {
		return err
	}
	// verify any internal bots are also defined in external
	for name := range internal {
		_, ok := external[name]
		if ok {
			return fmt.Errorf("%s bot is defined in both internal and external configurations", name)
		}
	}

	// validate aliases in external config
	for name, bot := range external {
		alias := bot.Alias
		if alias != "" {
			nextBot, ok := external[alias]
			if !ok {
				return fmt.Errorf("%s uses alias %s that is not defined in the external configs", name, alias)
			}
			if nextBot.Alias != "" {
				return fmt.Errorf("%s cannot have nested aliases in bot configurations", name)
			}
		}
	}

	// combine maps and validate each device
	err = json.Unmarshal(externalBotConfigs, &botConfigs)
	if err != nil {
		return err
	}
	err = json.Unmarshal(internalBotConfigs, &botConfigs)
	if err != nil {
		return err
	}
	for name, bot := range botConfigs {
		alias := bot.Alias
		if alias != "" {
			nextBot, ok := botConfigs[alias]
			if !ok {
				return fmt.Errorf("%s uses alias %s that is not defined in either config", name, alias)
			}
			if nextBot.Alias != "" {
				return fmt.Errorf("%s cannot have nested aliases in bot configurations", name)
			}
			if bot.Browser != "" || bot.Builder != "" || len(bot.Dimensions) > 0 ||
				bot.Repo != "" || bot.SwarmingServer != "" {
				return fmt.Errorf("%s defines both an alias and other fields. Do one or the other.", name)
			}
			continue
		}

		// verify bot has all other parameters defined
		if bot.Browser == "" {
			return fmt.Errorf("%s is missing browser configs", name)
		} else if bot.Builder == "" {
			return fmt.Errorf("%s is missing builder configs", name)
		} else if len(bot.Dimensions) == 0 {
			return fmt.Errorf("%s does not have any dimensions", name)
		} else if bot.Repo == "" {
			return fmt.Errorf("%s is missing repository configs", name)
		} else if bot.SwarmingServer == "" {
			return fmt.Errorf("%s is missing swarming server configs", name)
		}
	}

	return nil
}
