// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/settings"
	"golang.org/x/net/context"
)

// globalConfigSettingsKey is the settings key for the Coordinator instance's
// GlobalConfig.
const globalConfigSettingsKey = "LogDogCoordinatorGlobalSettings"

// Settings is the LogDog Coordinator auxiliary (runtime) settings. These are
// stored within a given datastore instance, rather than in luci-config, due
// to their sensitivity.
type Settings struct {
	// BigTableServiceAccountJSON, if not empty, is the service account JSON file
	// data that will be used for BigTable access.
	//
	// TODO(dnj): Remove this option once Cloud BigTable has cross-project ACLs.
	BigTableServiceAccountJSON []byte `json:"bigTableServiceAccountJson"`
}

// Load populates the settings instance from the stored settings.
//
// If no settings are stored, an empty Settings instance will be loaded and
// this will return nil.
//
// An error will be returned if an operation that is expected to succeed fails.
func (s *Settings) Load(c context.Context) error {
	var loadMe Settings

	// Load additional global config from settings. If it's missing, that's fine,
	// since its fields are all optional.
	if err := settings.Get(c, globalConfigSettingsKey, &loadMe); err != nil {
		// The settings are missing, so let's install the empty settings.
		if err != settings.ErrNoSettings {
			log.WithError(err).Errorf(c, "Failed to load global config from settings.")
			return err
		}

		if err := settings.Set(c, globalConfigSettingsKey, &loadMe, "application", "initial empty config"); err != nil {
			log.WithError(err).Warningf(c, "Failed to initialize empty config.")
		}
	}

	*s = loadMe
	return nil
}

// Store stores the new global configuration.
func (s *Settings) Store(c context.Context, why string) error {
	id := auth.CurrentIdentity(c)
	log.Fields{
		"identity": id,
		"reason":   why,
	}.Infof(c, "Updating global configuration.")
	return settings.Set(c, globalConfigSettingsKey, s, string(id), why)
}

// Validate validates the correctness of this configuration, returning an error
// if it's invalid.
//
// Note that only the contents saved to settings are validated. The read-only
// configuration is not.
func (s Settings) Validate() error {
	// NOTE: This currently does nothing. However, we're leaving it in here so
	// external callers can still work validation into their workflow in case
	// additional parameters are added in the future.
	return nil
}
