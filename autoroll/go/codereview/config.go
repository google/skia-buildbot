package codereview

import (
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/util"
)

const (
	// GERRIT_CONFIG_ANDROID is a Gerrit server configuration used by
	// Android and related projects.
	GERRIT_CONFIG_ANDROID = "android"

	// GERRIT_CONFIG_ANGLE is a Gerrit server configuration used by ANGLE.
	GERRIT_CONFIG_ANGLE = "angle"

	// GERRIT_CONFIG_CHROMIUM is a Gerrit server configuration used by
	// Chromium and related projects.
	GERRIT_CONFIG_CHROMIUM = "chromium"

	// GERRIT_CONFIG_CHROMIUM_NO_CQ is a Gerrit server configuration used by
	// Chromium for projects with no Commit Queue.
	GERRIT_CONFIG_CHROMIUM_NO_CQ = "chromium-no-cq"

	// GERRIT_CONFIG_LIBASSISTANT is a Gerrit server configuration used by
	// libassistant.
	GERRIT_CONFIG_LIBASSISTANT = "libassistant"
)

var (
	// GerritConfigs maps Gerrit config names to gerrit.Configs.
	GerritConfigs = map[config.GerritConfig_Config]*gerrit.Config{
		config.GerritConfig_ANDROID:        gerrit.CONFIG_ANDROID,
		config.GerritConfig_ANGLE:          gerrit.CONFIG_ANGLE,
		config.GerritConfig_CHROMIUM:       gerrit.CONFIG_CHROMIUM,
		config.GerritConfig_CHROMIUM_NO_CQ: gerrit.CONFIG_CHROMIUM_NO_CQ,
		config.GerritConfig_LIBASSISTANT:   gerrit.CONFIG_LIBASSISTANT,
	}
)

// CodeReviewConfig provides generalized configuration information for a code
// review service.
type CodeReviewConfig interface {
	util.Validator
}
