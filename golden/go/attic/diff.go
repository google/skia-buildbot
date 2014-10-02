package attic

type TestRunResult struct {
	Key       map[string]string
	ImageHash string
}

type TestExpectation struct {
	Key              map[string]string
	ValidImageHashes map[string]bool
}

type TestRunFailure struct {
	Actual      TestRunResult
	Expectation *TestExpectation
}

func DiffTestRunVsExpectations(actuals []TestRunResult, expectations []*TestExpectation, primaryKey []string) []TestRunFailure {
	// gather all the failed test runs
	result := make([]TestRunFailure, 0, len(actuals))

	// get a map fo the expectations for faster lookup keyed by primary
	// key (test identifier)
	expectationsMap := make(map[string]*TestExpectation)
	for _, e := range expectations {
		expectationsMap[MapToStrKey(e.Key)] = e
	}

	// iterate over the actuals
	for _, act := range actuals {
		k := NewMultiKey(SubMap(act.Key, primaryKey))
		exp, ok := expectationsMap[k.Key()]
		if ok {
			_, ok = exp.ValidImageHashes[act.ImageHash]
		}

		// Either we could not find the test or there is no matching hash.
		if !ok {
			result = append(result, TestRunFailure{act, exp})
		}
	}

	return result
}
