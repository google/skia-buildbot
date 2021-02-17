package codereview

import (
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/util"
)

var (
	// GerritConfigs maps Gerrit config names to gerrit.Configs.
	GerritConfigs = map[config.GerritConfig_Config]*gerrit.Config{
		config.GerritConfig_ANDROID:             gerrit.ConfigAndroid,
		config.GerritConfig_ANGLE:               gerrit.ConfigANGLE,
		config.GerritConfig_CHROMIUM:            gerrit.ConfigChromium,
		config.GerritConfig_CHROMIUM_NO_CQ:      gerrit.ConfigChromiumNoCQ,
		config.GerritConfig_CHROMIUM_BOT_COMMIT: gerrit.ConfigChromiumBotCommit,
		config.GerritConfig_LIBASSISTANT:        gerrit.ConfigLibAssistant,
	}
)

// CodeReviewConfig provides generalized configuration information for a code
// review service.
type CodeReviewConfig interface {
	util.Validator
}
