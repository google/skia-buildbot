# RAG Evaluation Framework

This framework allows for automated retrieval evaluation of the History RAG system. It is designed
to verify the quality of the generated topic indices and embeddings by running standardized queries
against a "golden" evaluation set.

## How it Works

The evaluation follows these steps:

1. **In-Memory Ingestion**: The provided ZIP file (containing `embeddings.npy`, `index.pkl`, and
   `topics/*.json`) is ingested into a fast `InMemoryTopicStore`. This avoids the need for a live
   Spanner instance.
2. **Query Execution**: Each query in the evaluation set is converted into an embedding vector
   using the Gemini API.
3. **Retrieval**: The system searches for the top 5 most similar chunks in the memory store.
4. **Metric Calculation**: The retrieved topic names are compared against the expected names to
   calculate **Recall@5** and **MRR**.

## Metrics Explained

### Recall@5 (Coverage)

Recall@5 measures if the "correct" topic was found anywhere in the top 5 results.

- **Formula**: `(Number of relevant topics found in top 5) / (Total number of relevant topics)`
- **Significance**: If Recall@5 is low, it means the LLM is never seeing the relevant information
  because it wasn't retrieved in the first place. For a high-quality index, this should
  be **> 0.80**.

### MRR - Mean Reciprocal Rank (Ranking Quality)

MRR measures how high up the first correct topic appears in the results.

- **Formula**: `1 / Rank of the first relevant result` (e.g., 1.0 if at rank 1, 0.5 if at rank 2).
- **Significance**: A high MRR indicates that the system is not only finding the right data but
  ranking it at the very top. High ranking accuracy leads to better summaries because LLMs often
  prioritize the first pieces of context they read. Aim for **> 0.70**.

## How to Run the Evaluation

1.  **Prepare your Evaluation Set**: Create a JSON file (e.g., `eval_set.json`) with the following
    structure:

        ```json
        {
          "test_cases": [
            {
              "query": "How do I handle authentication?",
              "expected_topic_names": ["AuthMiddleware Implementation"]
            }
          ]
        }
        ```

2.  **Set your API Key**:

    ```bash
    export GEMINI_API_KEY="your-api-key"
    ```

3.  **Execute the Tool**:
    ```bash
    bazelisk run //rag/go/eval/eval_tool --
      --zip_path=/path/to/data.zip
      --eval_set_path=./eval_set.json
      --config_path=./rag/configs/demo.json
    ```

## Interpreting Results

- **Recall@5 = 1.0, MRR = 1.0**: Perfect retrieval. The top result is exactly what was expected.
- **Recall@5 = 1.0, MRR = 0.2**: The system found the right data, but it was buried at the 5th
  position. This suggests that while the embeddings are somewhat relevant, the ranking needs
  improvement (possibly due to noisy chunks).
- **Recall@5 = 0.0**: Total failure. The embedding for the query is not matching the topic chunks
  at all. This usually indicates a mismatch in the embedding model or poor chunking strategy.
