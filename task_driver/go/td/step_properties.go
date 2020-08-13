package td

import "go.skia.org/infra/task_driver/go/td/properties"

// Props is a convenience function provided so that the caller does not have to
// import the "properties" package.
func Props(name string) *properties.StepProperties {
	return properties.Props(name)
}
