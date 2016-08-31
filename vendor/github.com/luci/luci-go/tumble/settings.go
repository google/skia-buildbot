// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"
	"html/template"
	"strconv"

	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/settings"
	"golang.org/x/net/context"
)

const (
	settingDisabled = "disabled"
	settingEnabled  = "enabled"
)

// settingsUIPage is a UI page to configure a static Tumble configuration.
type settingsUIPage struct {
	settings.BaseUIPage
}

func (settingsUIPage) Title(c context.Context) (string, error) {
	return "Tumble settings", nil
}

func (settingsUIPage) Overview(c context.Context) (template.HTML, error) {
	return template.HTML(`<p>Configuration parameters for the
<a href="https://github.com/luci/luci-go/tree/master/tumble">tumble
service</a> can be found in its
<a href="https://godoc.org/github.com/luci/luci-go/tumble">
documentation</a>.</p>.`), nil
}

func (settingsUIPage) Fields(c context.Context) ([]settings.UIField, error) {
	return []settings.UIField{
		{
			ID:          "NumShards",
			Title:       "Number of shards to use",
			Type:        settings.UIFieldText,
			Placeholder: strconv.FormatUint(defaultConfig.NumShards, 10),
			Validator:   intValidator(true),
		},
		{
			ID:          "TemporalMinDelay",
			Title:       "Temporal minimum delay (s, m, h)",
			Type:        settings.UIFieldText,
			Placeholder: defaultConfig.TemporalMinDelay.String(),
			Validator:   validateDuration,
		},
		{
			ID:          "TemporalRoundFactor",
			Title:       "Temporal round factor, for batching (s, m, h)",
			Type:        settings.UIFieldText,
			Placeholder: defaultConfig.TemporalRoundFactor.String(),
			Validator:   validateDuration,
		},
		{
			ID:          "DustSettleTimeout",
			Title:       "Amount of time to wait for datastore to settle in between mutation rounds",
			Type:        settings.UIFieldText,
			Placeholder: defaultConfig.DustSettleTimeout.String(),
			Validator:   validateDuration,
		},
		{
			ID:          "NumGoroutines",
			Title:       "Number of goroutines per shard",
			Type:        settings.UIFieldText,
			Placeholder: strconv.Itoa(defaultConfig.NumGoroutines),
			Validator:   intValidator(true),
		},
		{
			ID:          "ProcessMaxBatchSize",
			Title:       "Number of mutations to include per commit (negative for unlimited)",
			Type:        settings.UIFieldText,
			Placeholder: strconv.Itoa(int(defaultConfig.ProcessMaxBatchSize)),
			Validator:   intValidator(false),
		},
		{
			ID:             "DelayedMutations",
			Title:          "Delayed mutations (index MUST be present)",
			Type:           settings.UIFieldChoice,
			ChoiceVariants: []string{settingDisabled, settingEnabled},
		},
		{
			ID:             "Namespaced",
			Title:          "Operate in namespaced mode",
			Type:           settings.UIFieldChoice,
			ChoiceVariants: []string{settingDisabled, settingEnabled},
		},
	}, nil
}

func (settingsUIPage) ReadSettings(c context.Context) (map[string]string, error) {
	var cfg Config
	switch err := settings.GetUncached(c, baseName, &cfg); err {
	case nil:
		break
	case settings.ErrNoSettings:
		logging.WithError(err).Infof(c, "No settings available, using defaults.")
		cfg = defaultConfig
	default:
		return nil, err
	}

	values := map[string]string{}

	if cfg.NumShards != 0 {
		values["NumShards"] = strconv.FormatUint(cfg.NumShards, 10)
	}
	if cfg.TemporalMinDelay != 0 {
		values["TemporalMinDelay"] = cfg.TemporalMinDelay.String()
	}
	if cfg.TemporalRoundFactor != 0 {
		values["TemporalRoundFactor"] = cfg.TemporalRoundFactor.String()
	}
	if cfg.DustSettleTimeout != 0 {
		values["DustSettleTimeout"] = cfg.DustSettleTimeout.String()
	}
	if cfg.NumGoroutines != 0 {
		values["NumGoroutines"] = strconv.Itoa(cfg.NumGoroutines)
	}
	if cfg.ProcessMaxBatchSize != 0 {
		values["ProcessMaxBatchSize"] = strconv.FormatInt(int64(cfg.ProcessMaxBatchSize), 10)
	}

	values["DelayedMutations"] = getToggleSetting(cfg.DelayedMutations)
	values["Namespaced"] = getToggleSetting(cfg.Namespaced)

	return values, nil
}

func (settingsUIPage) WriteSettings(c context.Context, values map[string]string, who, why string) error {
	cfg := Config{}

	var err error
	if v := values["NumShards"]; v != "" {
		cfg.NumShards, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("could not parse NumShards: %v", err)
		}
	}
	if v := values["TemporalMinDelay"]; v != "" {
		cfg.TemporalMinDelay, err = clockflag.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("could not parse TemporalMinDelay: %v", err)
		}
	}
	if v := values["TemporalRoundFactor"]; v != "" {
		cfg.TemporalRoundFactor, err = clockflag.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("could not parse TemporalRoundFactor: %v", err)
		}
	}
	if v := values["DustSettleTimeout"]; v != "" {
		cfg.DustSettleTimeout, err = clockflag.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("could not parse DustSettleTimeout: %v", err)
		}
	}
	if v := values["NumGoroutines"]; v != "" {
		cfg.NumGoroutines, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("could not parse NumGoroutines: %v", err)
		}
	}
	if v := values["ProcessMaxBatchSize"]; v != "" {
		val, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("could not parse ProcessMaxBatchSize: %v", err)
		}
		cfg.ProcessMaxBatchSize = int32(val)
	}
	cfg.DelayedMutations = values["DelayedMutations"] == settingEnabled
	cfg.Namespaced = values["Namespaced"] == settingEnabled

	return settings.SetIfChanged(c, baseName, &cfg, who, why)
}

func intValidator(positive bool) func(string) error {
	return func(v string) error {
		if v == "" {
			return nil
		}
		i, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid integer %q - %s", v, err)
		}
		if positive && i <= 0 {
			return fmt.Errorf("value %q must be positive", v)
		}
		return nil
	}
}

func validateDuration(v string) error {
	if v == "" {
		return nil
	}

	var cf clockflag.Duration
	if err := cf.Set(v); err != nil {
		return fmt.Errorf("bad duration %q - %s", v, err)
	}
	if cf <= 0 {
		return fmt.Errorf("duration %q must be positive", v)
	}
	return nil
}

func getToggleSetting(v bool) string {
	if v {
		return settingEnabled
	}
	return settingDisabled
}

func init() {
	settings.RegisterUIPage("tumble", settingsUIPage{})
}
