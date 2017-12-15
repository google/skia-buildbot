package search

func (s *SearchAPI) SearchTryjobs(q *Query) (*NewSearchResponse, error) {
	// Keep track if we are including reference diffs. This is going to be true
	// for the majority of queries.
	getRefDiffs := !q.NoDiff
	isTryjobQuery := q.Issue > 0

	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	exp, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, err
	}
	idx := s.ixr.GetIndex()

	// TODO: issue exp change.

	// query the issue

	// Calculate differences by comparing to expectations of master and this issue.

	// filter the diff result.

	//

	//
	//
	//
	//  ^ SAME
	//
	//
	//
	//
	//
	allExp := exp

	// Unconditional query stage. Iterate through the tile and get an intermediate
	// representation that contains all the traces matching the queries.
	ret, err := s.queryIssue(q, allExp)
	if err != nil {
		return nil, err
	}

	// Get reference diffs unless it was specifically disabled.
	if getRefDiffs {
		// Diff stage: Compare all digests found in the previous stages and find
		// reference points (positive, negative etc.) for each digest.
		s.getReferenceDiffs(ret, q.Metric, q.Match, q.IncludeIgnores, idx, allExp)
		if err != nil {
			return nil, err
		}

		// Post-diff stage: Apply all filters that are relevant once we have
		// diff values for the digests.
		ret = s.afterDiffResultFilter(ret, q)
	}

	// Sort the digests and fill the ones that are going to be displayed with
	// additional data. Note we are returning all digests found, so we can do
	// bulk triage, but only the digests that are going to be shown are padded
	// with additional information.
	displayRet, offset := s.sortAndLimitDigests(q, ret, int(q.Offset), int(q.Limit))
	s.addParamsAndTraces(displayRet, inter, exp, idx)

	// Return all digests with the selected offset within the result set.
	return &NewSearchResponse{
		Digests: ret,
		Offset:  offset,
		Size:    len(displayRet),
		Commits: idx.GetTile(false).Commits,
	}, nil
}

func (s *SearchAPI) queryIssue(q *Query) (map[string]map[string]*srIntermediate, error) {

	return nil, nil
}
