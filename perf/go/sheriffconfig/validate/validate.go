package validate

import (
	"encoding/base64"
	"net/url"
	"regexp"

	"go.skia.org/infra/go/skerr"
	sheriff_configpb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"google.golang.org/protobuf/encoding/prototext"
)

// Does the following checks at Pattern level:
//   - The match and exclude strings are in correct formats.
//   - All pattern specify at least 1 key.
//   - Exclude patterns only specify 1 key.
//   - If a value starts with "~", check that the rest of the string is a
//     compileable Regex.
func validatePattern(pattern string, singleField bool) error {

	query, err := url.ParseQuery(pattern)
	if err != nil {
		return skerr.Fmt("Pattern '%s' has incorrect format: %s", pattern, err)
	}

	if len(query) == 0 {
		return skerr.Fmt("Pattern must have at least 1 key declared.")
	}

	if singleField && len(query) > 1 {
		return skerr.Fmt("Pattern must only have 1 key declared.")
	}

	for key, values := range query {
		if len(values) == 0 {
			return skerr.Fmt("Key must have at least 1 explicit value declared. Key: %s.", key)
		}
		for _, value := range values {
			if len(value) == 0 {
				return skerr.Fmt("Explicit value for key must be non-empty. Key: %s.", key)
			}
			if value[:1] == "~" {
				_, err := regexp.Compile(value[1:])
				if err != nil {
					return skerr.Fmt("Invalid Regex for '%s' key: %s.", key, value[1:])
				}
			}
		}
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

// Transform Base64 encoded data into SheriffConfig proto.
// LUCI Config returns content encoded in base64. It then needs to be
// Unmarshaled into Sheriff Config proto.
func DeserializeProto(encoded string) (*sheriff_configpb.SheriffConfig, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, skerr.Fmt("Failed to decode Base64 string: %s", err)
	}
	config := &sheriff_configpb.SheriffConfig{}

	err = prototext.Unmarshal(decoded, config)
	if err != nil {
		return nil, skerr.Fmt("Failed to unmarshal prototext: %s", err)
	}

	return config, nil
}
