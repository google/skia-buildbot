// Test utils provides util functions for gold cmd packages.
package validation

import (
	"fmt"
	"path/filepath"

	"go.skia.org/infra/golden/go/config"
)

type InstanceCategory []string

// List of primary instances for Gold.
var PrimaryInstances InstanceCategory = []string{
	"angle",
	"chrome",
	"cros-tast",
	"eskia",
	"flutter",
	"flutter-engine",
	"koru",
	"lottie",
	"lottie-spec",
	"pdfium",
	"skia",
	"skia-infra",
}

// List of mirror instances for Gold.
var MirrorInstances InstanceCategory = []string{
	"chrome-public",
	"skia-public",
}

// Union of all instances.
var AllInstances InstanceCategory = append(MirrorInstances, PrimaryInstances...)

// ValidateServiceConfigs validates the instance configs in the repository to ensure that services
// are able to load them. This protects against service failures due to invalid/missing config
// values in production. Configs can be optional for instances, hence we require the list of
// instances to check for.
func ValidateServiceConfigs(serviceName string, instances InstanceCategory, configObj interface{}) error {
	for _, instance := range instances {
		// The common config for the instance is specified as <instance>.json5. Eg: skia.json5.
		commonConfigFileName := fmt.Sprintf("%s.json5", instance)
		commonConfigFile, err := filepath.Glob(filepath.Join("..", "..", "k8s-instances", instance, commonConfigFileName))
		if err != nil {
			return err
		}

		// The service specific config is specified as <instance>-<service>.json5. Eg: skia-frontend.json5
		serviceFileName := fmt.Sprintf("%s-%s.json5", instance, serviceName)
		serviceConfigFile, err := filepath.Glob(filepath.Join("..", "..", "k8s-instances", instance, serviceFileName))
		if err != nil {
			return err
		}

		// Check if we are able to load the config and return error if any.
		err = config.LoadFromJSON5(configObj, &commonConfigFile[0], &serviceConfigFile[0])
		if err != nil {
			return err
		}
	}

	return nil
}
