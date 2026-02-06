package eval

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/rag/go/genai"
	"go.skia.org/infra/rag/go/topicstore"
)

// TestCase represents a single evaluation query and its expected results.
type TestCase struct {
	Query              string   `json:"query"`
	ExpectedTopicNames []string `json:"expected_topic_names"`
	Category           string   `json:"category,omitempty"`
}

// EvaluationSet is a collection of TestCases.
type EvaluationSet struct {
	TestCases []TestCase `json:"test_cases"`
}

// EvalResult contains the performance metrics for a specific query.
type EvalResult struct {
	Query         string   `json:"query"`
	RecallAt5     float64  `json:"recall_at_5"`
	MRR           float64  `json:"mrr"`
	FoundNames    []string `json:"found_names"`
	ExpectedNames []string `json:"expected_names"`
	Passed        bool     `json:"passed"`
}

// SummaryReport contains the aggregated results of an evaluation run.
type SummaryReport struct {
	TotalQueries  int           `json:"total_queries"`
	MeanRecallAt5 float64       `json:"mean_recall_at_5"`
	MeanMRR       float64       `json:"mean_mrr"`
	Results       []*EvalResult `json:"results"`
}

// Evaluator performs the evaluation.
type Evaluator struct {
	genAiClient    genai.GenAIClient
	topicStore     topicstore.TopicStore
	embeddingModel string
	dimensionality int32
}

// NewEvaluator returns a new instance of Evaluator.
func NewEvaluator(genAiClient genai.GenAIClient, topicStore topicstore.TopicStore, embeddingModel string, dimensionality int32) *Evaluator {
	return &Evaluator{
		genAiClient:    genAiClient,
		topicStore:     topicStore,
		embeddingModel: embeddingModel,
		dimensionality: dimensionality,
	}
}

// Run performs the evaluation on the provided evaluation set.
func (e *Evaluator) Run(ctx context.Context, evalSet *EvaluationSet) (*SummaryReport, error) {
	report := &SummaryReport{
		TotalQueries: len(evalSet.TestCases),
	}

	var sumRecallAt5, sumMRR float64

	for _, tc := range evalSet.TestCases {
		res, err := e.evaluateTestCase(ctx, tc)
		if err != nil {
			sklog.Errorf("Error evaluating test case '%s': %v", tc.Query, err)
			continue
		}
		report.Results = append(report.Results, res)
		sumRecallAt5 += res.RecallAt5
		sumMRR += res.MRR
	}

	if report.TotalQueries > 0 {
		report.MeanRecallAt5 = sumRecallAt5 / float64(report.TotalQueries)
		report.MeanMRR = sumMRR / float64(report.TotalQueries)
	}

	return report, nil
}

func (e *Evaluator) evaluateTestCase(ctx context.Context, tc TestCase) (*EvalResult, error) {
	// 1. Get embedding for the query.
	embedding, err := e.genAiClient.GetEmbedding(ctx, e.embeddingModel, e.dimensionality, tc.Query)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// 2. Search for top topics.
	// We search for top 5 to calculate Recall@5.
	found, err := e.topicStore.SearchTopics(ctx, embedding, 5)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	res := &EvalResult{
		Query:         tc.Query,
		ExpectedNames: tc.ExpectedTopicNames,
		Passed:        false,
	}

	for _, f := range found {
		res.FoundNames = append(res.FoundNames, f.Title)
	}

	// 3. Calculate metrics.
	res.RecallAt5 = calculateRecall(res.FoundNames, tc.ExpectedTopicNames)
	res.MRR = calculateMRR(res.FoundNames, tc.ExpectedTopicNames)

	// A simple pass criteria: at least one expected topic is found in top 5.
	if res.RecallAt5 > 0 {
		res.Passed = true
	}

	return res, nil
}

func calculateRecall(found, expected []string) float64 {
	if len(expected) == 0 {
		return 1.0
	}
	count := 0
	expectedMap := make(map[string]bool)
	for _, name := range expected {
		expectedMap[strings.ToLower(name)] = true
	}
	for _, name := range found {
		if expectedMap[strings.ToLower(name)] {
			count++
		}
	}
	return float64(count) / float64(len(expected))
}

func calculateMRR(found, expected []string) float64 {
	expectedMap := make(map[string]bool)
	for _, name := range expected {
		expectedMap[strings.ToLower(name)] = true
	}

	for i, name := range found {
		if expectedMap[strings.ToLower(name)] {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// LoadEvaluationSet loads the evaluation set from a JSON file.
func LoadEvaluationSet(filePath string) (*EvaluationSet, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var evalSet EvaluationSet
	if err := json.Unmarshal(content, &evalSet); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &evalSet, nil
}
