# Decoupled Comment RAG API Service

This directory contains the decoupled, standalone code for the generic Comment Retrieval-Augmented Generation (RAG) API service.

## Running the Server

You can compile the standalone binary and run the server in one command:

```bash
make run-comment-api
```

By default, this loads coordinates from `./configs/demo.json` pointing to the Spanner database `ipcreviewindex` inside the GCP project `pasthana-pd-ai-keys`.

## Environment Variables

For Gemini embedding access, ensure that you have set your API key in your shell environment:

```bash
export GEMINI_API_KEY="your-api-key"
```
