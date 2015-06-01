package config

type Common struct {
	ResourcePath          string
	FuzzOutputGSBucket    string
	DoOAuth               bool
	OAuthCacheFile        string
	OAuthClientSecretFile string
	Local                 bool
}

type FrontEnd struct {
	Port           string
	GraphiteServer string
	RedirectURL    string
	ForceLogin     bool
}

type Fuzzer struct {
	CachePath     string
	Indentation   int
	SkiaSourceDir string
}

type Generator struct {
	Weight      int
	Debug       bool
	ExtraParams map[string]string
}

var Config struct {
	Common     Common
	FrontEnd   FrontEnd
	Fuzzer     Fuzzer
	Generators map[string]Generator
}
