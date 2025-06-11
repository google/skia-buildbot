# Chatbot for Google Chat

This directory contains a Google Chat application that uses the MCP client library. It
is designed to be deployed as a Google Workspace Add-on using `clasp`.

## Prerequisites

- [clasp](https://github.com/google/clasp) installed and authenticated.
- A Google Cloud project with the "Google Chat API" enabled.
- An Apps Script project linked to your Cloud project.

## Deployment

The deployment process involves building the TypeScript code with Bazel, preparing a
`dist` directory with the compiled JavaScript and the manifest file, and then pushing
that directory to Google Apps Script with `clasp`.

### 1. Build the Chatbot

The `ts_library` rule in the `BUILD.bazel` file compiles the TypeScript source
(`main.ts`) into JavaScript.

Run the following command from the root of the repository to build the chatbot:

```bash
bazelisk build //mcp/client/chatbot:main_ts_lib
```

### 2. Prepare Deployment Directory

`clasp` pushes the files from your current directory. We need to create a directory with
the compiled JavaScript and the Apps Script manifest.

The following commands create a `dist` directory, copy the necessary files into it, and
prepare it for `clasp`. Note that Apps Script requires the main JavaScript file to be
named `Code.js`.

```bash
# Create a distribution directory
mkdir -p mcp/client/chatbot/dist

# Copy the compiled JavaScript and rename it to Code.js
cp _bazel_bin/mcp/client/chatbot/main.js mcp/client/chatbot/dist/Code.js

# Copy the Apps Script manifest
cp mcp/client/chatbot/appsscript.json mcp/client/chatbot/dist/
```

### 3. Deploy with Clasp

Navigate into the `dist` directory and push the files using `clasp`.

```bash
cd mcp/client/chatbot/dist
clasp push
```

This will upload the `Code.js` and `appsscript.json` files to your linked Apps Script
project. From there, you can deploy it from the Apps Script editor.
