package config

type FrontEnd struct {
	Port           string
	ResourcePath   string
	GraphiteServer string
}

var Fuzzer struct {
	FrontEnd FrontEnd
}
