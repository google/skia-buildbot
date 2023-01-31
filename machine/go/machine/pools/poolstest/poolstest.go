package poolstest

import "go.skia.org/infra/machine/go/machineserver/config"

var PoolConfigForTesting = config.InstanceConfig{
	Pools: []config.Pool{
		{
			Name:  "SkiaInternal",
			Regex: "^skia-i-",
		},
		{
			Name:  "Skia",
			Regex: "^skia-",
		},
	},
}
