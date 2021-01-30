package sktest

// TestingT is an interface which is compatible with testing.T and testing.B,
// used so that we don't have to import the "testing" package except in _test.go
// files.
type TestingT interface {
	Cleanup(func())
	Error(...interface{})
	Errorf(string, ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Helper()
	Log(...interface{})
	Logf(string, ...interface{})
	Name() string
	Skip(...interface{})
	SkipNow()
	Skipf(string, ...interface{})
	Skipped() bool
}
