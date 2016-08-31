// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

// ToQuery generates a new GraphQuery.
//
// This generates a GraphQuery that queries for any Attempts which are marked as
// Partial in the current GraphData.
func (g *GraphData) ToQuery() (ret *GraphQuery) {
	partials := map[string][]uint32{}
	for qid, qst := range g.Quests {
		// TODO(iannucci): handle q.Partial explicitly?
		for aid, atmpt := range qst.Attempts {
			if atmpt.Partial.Any() {
				partials[qid] = append(partials[qid], aid)
			}
		}
	}

	if len(partials) > 0 {
		ret = &GraphQuery{AttemptList: NewAttemptList(partials)}
	}
	return
}
