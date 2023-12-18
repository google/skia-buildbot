package urlprovider

// ParamsProvider provides an interface to manage param names
type ParamsProvider interface {
	// GetParamKey returns the param key for the given param name
	GetParamKey(paramName string) string
}

type DefaultParamsProvider struct{}

// GetParamKey returns the same value since default provider does not do any translation
func (prov *DefaultParamsProvider) GetParamKey(paramName string) string {
	return paramName
}

var _ ParamsProvider = &DefaultParamsProvider{}
