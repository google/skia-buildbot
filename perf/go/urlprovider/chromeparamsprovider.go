package urlprovider

import (
	"slices"
)

// ChromeParamsProvider provides params specific to chrome workflows.
type ChromeParamsProvider struct {
	IgnoreParams []string
	ParamsMap    map[string]string
}

// GetParamKey returns the param key for the given param name
func (prov *ChromeParamsProvider) GetParamKey(paramName string) string {
	// If the give param is in the ignore list, return empty.
	if prov.IgnoreParams != nil {
		if slices.Contains(prov.IgnoreParams, paramName) {
			return ""
		}
	}

	// Translate the given param if it is present in the params map
	if prov.ParamsMap != nil {
		if translatedName, ok := prov.ParamsMap[paramName]; ok {
			return translatedName
		}
	}

	return paramName
}

var _ ParamsProvider = &ChromeParamsProvider{}
