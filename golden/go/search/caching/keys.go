package caching

import "fmt"

func ByBlameKey(corpus string) string {
	return corpus + "_byblame"
}

// MatchingTracesKey returns a key to be used to cache the data based on the query context.
func MatchingTracesKey(queryContext MatchingTracesQueryContext) string {
	var digestType string
	if queryContext.IncludeUntriaged {
		digestType = "untriaged"
	} else if queryContext.IncludeNegative {
		digestType = "negative"
	} else if queryContext.IncludePositive {
		digestType = "positive"
	} else if queryContext.IncludeIgnored {
		digestType = "ignored"
	}
	return fmt.Sprintf("matchingTraces_%s_%s", queryContext.Corpus, digestType)
}
