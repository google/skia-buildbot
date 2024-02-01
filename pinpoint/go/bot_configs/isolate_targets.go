package bot_configs

import (
	_ "embed"
	"strings"

	"go.skia.org/infra/go/skerr"
	"gopkg.in/yaml.v3"
)

const performance_test_suite = "performance_test_suite"

//go:embed isolate_targets.yaml
var isolateTargets []byte

type TargetMaps struct {
	Benchmark map[string]string `yaml:"BenchmarkTargets"`
	Exact     map[string]string `yaml:"BotExactTargets"`
	Regex     map[string]string `yaml:"BotRegexTargets"`
}

// GetIsolateTarget retrieves the CAS target for chrome builds using the benchmark and bot as inputs
// Most isolate targets are device specific but there is one benchmark specific target.
func GetIsolateTarget(bot, benchmark string) (string, error) {
	// verify bot can be run on Pinpoint
	_, err := GetBotConfig(bot, true)
	if err != nil {
		return "", skerr.Wrapf(err, "Cannot get isolate target of bot %s which is unlisted in bot_configs", bot)
	}

	// unmarshal config to struct
	tm := TargetMaps{}
	err = yaml.Unmarshal(isolateTargets, &tm)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not unmarshal isolate_targets.yaml")
	}

	t, ok := tm.Benchmark[benchmark]
	if ok {
		return t, nil
	}

	t, ok = tm.Exact[bot]
	if ok {
		return t, nil
	}

	bot = strings.ToLower(bot)
	for k, t := range tm.Regex {
		if strings.Contains(bot, k) {
			return t, nil
		}
	}

	return performance_test_suite, nil
}
