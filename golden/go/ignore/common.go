package ignore

import (
	"net/url"

	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
)

func BuildRuleMatcher(rulesList []*IgnoreRule) (RuleMatcher, error) {
	if len(rulesList) == 0 {
		return noopRuleMatcher, nil
	}

	ignoreRules := make([]QueryRule, len(rulesList))
	for idx, rawRule := range rulesList {
		parsedQuery, err := url.ParseQuery(rawRule.Query)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ignoreRules[idx] = NewQueryRule(parsedQuery)
	}

	return func(params map[string]string) ([]*IgnoreRule, bool) {
		var result []*IgnoreRule

		for ruleIdx, rule := range ignoreRules {
			if rule.IsMatch(params) {
				result = append(result, rulesList[ruleIdx])
			}
		}

		return result, len(result) > 0
	}, nil
}

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store. It also returns the
// ignore rules for later matching.
func FilterIgnored(inputTile *tiling.Tile, ignores []*IgnoreRule) (*tiling.Tile, paramtools.ParamMatcher, error) {
	// Make a shallow copy with a new Traces map
	ret := &tiling.Tile{
		Traces:   map[tiling.TraceId]tiling.Trace{},
		ParamSet: inputTile.ParamSet,
		Commits:  inputTile.Commits,

		Scale:     inputTile.Scale,
		TileIndex: inputTile.TileIndex,
	}

	// Then, add any traces that don't match any ignore rules
	ignoreQueries, err := ToQuery(ignores)
	if err != nil {
		return nil, nil, err
	}
nextTrace:
	for id, tr := range inputTile.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				continue nextTrace
			}
		}
		ret.Traces[id] = tr
	}

	ignoreRules := make([]paramtools.ParamSet, len(ignoreQueries))
	for idx, q := range ignoreQueries {
		ignoreRules[idx] = paramtools.ParamSet(q)
	}
	return ret, ignoreRules, nil
}
