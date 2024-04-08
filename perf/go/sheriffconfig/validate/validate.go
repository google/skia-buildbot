package validate

import (
	"regexp"

	"go.skia.org/infra/go/skerr"
	sheriff_configpb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
)

// Does the following checks at Pattern level:
//   - At least 1 field is populated.
//   - If singleField is true, only 1 field must be populated. This is used by
//     exclude patterns.
//   - If field value starts with "~", check that the rest of the string is a
//     compileable Regex.
func validatePattern(pattern *sheriff_configpb.Pattern, singleField bool) error {

	pr := pattern.ProtoReflect()
	fields := pr.Descriptor().Fields()

	nonEmptyFields := 0

	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		value := pr.Get(field).String()
		if value == "" {
			continue
		}

		nonEmptyFields += 1

		if value[:1] == "~" {
			_, err := regexp.Compile(value[1:])
			if err != nil {
				return skerr.Fmt("Invalid Regex for '%s' field: %s.", field.Name(), value[1:])
			}
		}
	}
	if nonEmptyFields < 1 {
		return skerr.Fmt("Pattern must have at least 1 explicit field declared.")
	}
	if singleField && nonEmptyFields > 1 {
		return skerr.Fmt("Pattern must only have 1 explicit field declared.")
	}
	return nil
}

// Does the following checks at Anomaly Config level:
// - At least 1 matching pattern exists.
func validateAnomalyConfig(ac *sheriff_configpb.AnomalyConfig) error {
	if len(ac.Rules.Match) == 0 {
		return skerr.Fmt("Anomaly config must have at least one match pattern.")
	}

	for i, pattern := range ac.Rules.Match {

		err := validatePattern(pattern, false)
		if err != nil {
			return skerr.Fmt("Error for Match Pattern at index %d: %s.", i, err)
		}
	}

	for i, pattern := range ac.Rules.Exclude {

		err := validatePattern(pattern, true)
		if err != nil {
			return skerr.Fmt("Error for Exclude Pattern at index %d: %s.", i, err)
		}

	}

	return nil
}

// Does the following checks at subscription level:
// - No missing Name, ContactEmail or BugComponent.
// - At least 1 Anomaly Config defined.
func validateSubscription(sub *sheriff_configpb.Subscription) error {

	if sub.Name == "" {
		return skerr.Fmt("Missing name.")
	}

	if sub.ContactEmail == "" {
		return skerr.Fmt("Subscription '%s' is missing contact_email.", sub.Name)
	}

	if sub.BugComponent == "" {
		return skerr.Fmt("Subscription '%s' is missing bug_component.", sub.Name)
	}

	if len(sub.AnomalyConfigs) == 0 {
		return skerr.Fmt("Subscription '%s' must have at least one Anomaly Config.", sub.Name)
	}

	for i, anomalyConfig := range sub.AnomalyConfigs {
		err := validateAnomalyConfig(anomalyConfig)
		if err != nil {
			return skerr.Fmt("Error for Anomaly Config at index %d: %s.", i, err)
		}
	}

	return nil
}

// Does the following checks at config level:
// - There's at least 1 subscription defined.
// - All subscriptions have unique names.
func ValidateConfig(config *sheriff_configpb.SheriffConfig) error {
	if len(config.Subscriptions) == 0 {
		return skerr.Fmt("Config must have at least one Subscription.")
	}

	namesSeen := make(map[string]bool)
	for i, sub := range config.Subscriptions {

		// Check if we have duplicate subscription names.
		if _, exists := namesSeen[sub.Name]; exists {
			return skerr.Fmt("Found duplicated subscription name: %s. Names must be unique.", sub.Name)
		}
		namesSeen[sub.Name] = true

		err := validateSubscription(sub)
		if err != nil {
			return skerr.Fmt("Error for Subscription at index %d: %s.", i, err)
		}
	}

	return nil
}
