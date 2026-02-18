package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/rag/go/config"
	"go.skia.org/infra/rag/go/eval"
	"go.skia.org/infra/rag/go/filereaders/zip"
	"go.skia.org/infra/rag/go/genai"
	"go.skia.org/infra/rag/go/ingest/history"
	"go.skia.org/infra/rag/go/topicstore"
)

const (
	embeddingFileName = "embeddings.npy"
	indexFileName     = "index.pkl"
	topicsDirName     = "topics"
	geminiApiKeyEnv   = "GEMINI_API_KEY"
)

func main() {
	zipPath := flag.String("zip_path", "", "Path to the input zip file.")
	evalSetPath := flag.String("eval_set_path", "", "Path to the evaluation set JSON file.")
	configPath := flag.String("config_path", "./configs/demo.json", "Path to the API server config file.")
	flag.Parse()

	if *zipPath == "" || *evalSetPath == "" {
		sklog.Fatal("--zip_path and --eval_set_path are required.")
	}

	sklogimpl.SetLogger(stdlogging.New(os.Stdout))
	ctx := context.Background()

	// 1. Load config
	cfg, err := config.NewApiServerConfigFromFile(*configPath)
	if err != nil {
		sklog.Fatalf("Error loading config: %v", err)
	}

	// 2. Setup stores and ingester
	// Note: We don't need a real blamestore for topic evaluation.
	topicStore := topicstore.NewInMemoryTopicStore()
	ingester := history.New(nil, topicStore, cfg.OutputDimensionality, cfg.UseRepositoryTopics, cfg.DefaultRepoName)

	// 3. Extract ZIP and Ingest
	content, err := os.ReadFile(*zipPath)
	if err != nil {
		sklog.Fatalf("Error reading zip file: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "rag-eval-*")
	if err != nil {
		sklog.Fatalf("Error creating temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			sklog.Errorf("Error cleaning up temp dir: %v", err)
		}
	}()

	sklog.Infof("Extracting %s to %s", *zipPath, tempDir)
	if err := zip.ExtractZipData(content, tempDir); err != nil {
		sklog.Fatalf("Error extracting zip: %v", err)
	}

	embeddingFilePath := filepath.Join(tempDir, embeddingFileName)
	indexFilePath := filepath.Join(tempDir, indexFileName)
	topicsDirPath := filepath.Join(tempDir, topicsDirName)

	sklog.Infof("Ingesting data into memory store...")
	if err := ingester.IngestTopics(ctx, topicsDirPath, embeddingFilePath, indexFilePath); err != nil {
		sklog.Fatalf("Error ingesting topics: %v", err)
	}

	// 4. Setup Evaluator
	apiKey := os.Getenv(geminiApiKeyEnv)
	if apiKey == "" {
		sklog.Fatalf("%s environment variable is not set.", geminiApiKeyEnv)
	}
	genAiClient, err := genai.NewLocalGeminiClient(ctx, apiKey)
	if err != nil {
		sklog.Fatalf("Error creating Gemini client: %v", err)
	}

	evaluator := eval.NewEvaluator(genAiClient, topicStore, cfg.QueryEmbeddingModel, int32(cfg.OutputDimensionality))

	// 5. Load Eval Set and Run
	evalSet, err := eval.LoadEvaluationSet(*evalSetPath)
	if err != nil {
		sklog.Fatalf("Error loading eval set: %v", err)
	}

	sklog.Infof("Running evaluation with %d test cases...", len(evalSet.TestCases))
	report, err := evaluator.Run(ctx, evalSet)
	if err != nil {
		sklog.Fatalf("Error running evaluation: %v", err)
	}

	// 6. Print Report
	printReport(report)
}

func printReport(report *eval.SummaryReport) {
	fmt.Println("--- Evaluation Results ---")
	fmt.Printf("Total Queries:    %d", report.TotalQueries)
	fmt.Printf("Mean Recall@5:    %.4f", report.MeanRecallAt5)
	fmt.Printf("Mean MRR:         %.4f", report.MeanMRR)
	fmt.Println("--------------------------")

	for _, res := range report.Results {
		status := "✅ PASS"
		if !res.Passed {
			status = "❌ FAIL"
		}
		fmt.Printf("%s | Query: %s", status, res.Query)
		fmt.Printf("   Recall@5: %.2f | MRR: %.2f", res.RecallAt5, res.MRR)
		if !res.Passed {
			fmt.Printf("   Expected: %v", res.ExpectedNames)
			fmt.Printf("   Found   : %v", res.FoundNames)
		}
		fmt.Println()
	}
	fmt.Println("--------------------------")
}
