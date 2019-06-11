package ignore

import "net/url"

func BuildRuleMatcher(store IgnoreStore) (RuleMatcher, error) {
	rulesList, err := store.List()
	if err != nil {
		return noopRuleMatcher, err
	}

	ignoreRules := make([]QueryRule, len(rulesList))
	for idx, rawRule := range rulesList {
		parsedQuery, err := url.ParseQuery(rawRule.Query)
		if err != nil {
			return noopRuleMatcher, err
		}
		ignoreRules[idx] = NewQueryRule(parsedQuery)
	}

	return func(params map[string]string) ([]*IgnoreRule, bool) {
		result := []*IgnoreRule{}

		for ruleIdx, rule := range ignoreRules {
			if rule.IsMatch(params) {
				result = append(result, rulesList[ruleIdx])
			}
		}

		return result, len(result) > 0
	}, nil
}
