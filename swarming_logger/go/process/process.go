package process

/*
   Utilities for processing Swarming task logs.
*/

import (
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"fmt"
	"sort"
	"strings"
)

type token struct {
	token string
	count int
}

type tokenSlice []token

func (t tokenSlice) Len() int           { return len(t) }
func (t tokenSlice) Less(i, j int) bool { return t[i].count > t[j].count }
func (t tokenSlice) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

func TokenizeLog(log []byte) ([]string, error) {
	// Just blindly split on whitespace.
	return strings.Fields(string(log)), nil
}

func CountTokens(tokens []string) []token {
	counts := make(map[string]int, len(tokens))
	for _, t := range tokens {
		counts[t]++
	}
	tokSlice := make([]token, len(counts))
	i := 0
	for k, v := range counts {
		tokSlice[i].token = k
		tokSlice[i].count = v
		i++
	}
	sort.Sort(tokenSlice(tokSlice))
	return tokSlice
}

func WriteTokenCountsCsv(tokens []token) ([]byte, error) {
	records := [][]string{
		{"token", "count"},
	}
	for _, tok := range tokens {
		records = append(records, []string{tok.token, fmt.Sprintf("%d", tok.count)})
	}
	rv := bytes.NewBuffer([]byte{})
	w := csv.NewWriter(rv)
	if err := w.WriteAll(records); err != nil {
		return nil, err
	}
	//sklog.Infof("%s", string(rv.Bytes()))
	return rv.Bytes(), nil
}

func TokenCountsCsv(log []byte) ([]byte, error) {
	tokens, err := TokenizeLog(log)
	if err != nil {
		return nil, err
	}
	counts := CountTokens(tokens)
	return WriteTokenCountsCsv(counts)
}

func NGrams(tokens []string, n int) [][]string {
	ngrams := make([][]string, 0, len(tokens)-n+1)
	for i := n; i < len(tokens); i++ {
		ngrams = append(ngrams, tokens[i-n:i])
	}
	return ngrams
}

func NGramsCsv(log []byte) ([]byte, error) {
	tokens, err := TokenizeLog(log)
	if err != nil {
		return nil, err
	}
	ngrams := NGrams(tokens, 2)
	rv := bytes.NewBuffer([]byte{})
	w := csv.NewWriter(rv)
	if err := w.WriteAll(ngrams); err != nil {
		return nil, err
	}
	return rv.Bytes(), nil
}

type MarkovChainReduction struct{}

func (m *MarkovChainReduction) DecodeInput(inp []byte) ([]byte, error) {
	records, err := csv.NewReader(bytes.NewReader(inp)).ReadAll()
	if err != nil {
		return nil, err
	}
	// Create a 2D map of tokens to counts.
	counts := make(map[string]map[string]int64, 1000)
	for _, record := range records {
		if len(record) != 2 {
			return nil, fmt.Errorf("Invalid format; expected 2-grams.")
		}
		subMap, ok := counts[record[0]]
		if !ok {
			subMap = make(map[string]int64, 1000)
			counts[record[0]] = subMap
		}
		subMap[record[1]]++
	}
	buf := bytes.NewBuffer([]byte{})
	if err := gob.NewEncoder(buf).Encode(counts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *MarkovChainReduction) Reduce(a, b []byte) ([]byte, error) {
	counts1 := make(map[string]map[string]int64, 1000)
	if err := gob.NewDecoder(bytes.NewReader(a)).Decode(&counts1); err != nil {
		return nil, err
	}
	counts2 := make(map[string]map[string]int64, 1000)
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&counts2); err != nil {
		return nil, err
	}

	// Merge the two maps.
	for term1, subMap2 := range counts2 {
		for term2, count := range subMap2 {
			subMap1, ok := counts1[term1]
			if !ok {
				subMap1 = make(map[string]int64, 1000)
				counts1[term1] = subMap1
			}
			subMap1[term2] += count
		}
	}
	buf := bytes.NewBuffer([]byte{})
	if err := gob.NewEncoder(buf).Encode(counts1); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *MarkovChainReduction) EncodeResult(inp []byte) ([]byte, error) {
	counts := make(map[string]map[string]int64, 1000)
	if err := gob.NewDecoder(bytes.NewReader(inp)).Decode(&counts); err != nil {
		return nil, err
	}
	totals := make(map[string]int64, len(counts))
	totalRecords := 0
	for term1, subMap := range counts {
		totalRecords += len(subMap)
		totalCount := int64(0)
		for _, count := range subMap {
			totalCount += count
		}
		totals[term1] = totalCount
	}
	records := make([][]string, 0, totalRecords)
	for term1, subMap := range counts {
		total := totals[term1]
		for term2, count := range subMap {
			probability := float64(count) / float64(total)
			records = append(records, []string{term1, term2, fmt.Sprintf("%4f", probability)})
		}
	}
	rv := bytes.NewBuffer([]byte{})
	w := csv.NewWriter(rv)
	if err := w.WriteAll(records); err != nil {
		return nil, err
	}
	//sklog.Infof("%s", rv.Bytes())
	return rv.Bytes(), nil
}
