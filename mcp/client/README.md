# MCP Client CLI

This document describes how to build and run the MCP Client CLI, a command-line interface
for interacting with MCP-compliant services using the Gemini API.

## Prerequisites

- Node.js and npm installed.
- A Google Gemini API Key.

## Setup

1.  **Install Dependencies:**
    From the root of the repository, run the following command to install the required
    Node.js packages:

    ```bash
    npm install
    ```

2.  **Configure API Key:**
    Create a `.env` file in the root of the repository. Add your Gemini API key to this file:
    ```
    GEMINI_API_KEY="YOUR_API_KEY_HERE"
    ```
    The client uses this file to load your API key securely.

## Building

The client and any services it connects to must be compiled using Bazel.

1.  **Build the Client:**

    ```bash
    bazelisk build //mcp/client:index_ts_lib
    ```

2.  **Build the Weather Service:**
    To test the client, build the example weather service:
    ```bash
    bazelisk build //mcp/services/weather:index_ts_lib
    ```
    These commands compile the TypeScript code and place the output in the `_bazel_bin` directory.

## Running the Client

To run the client, you execute its compiled script with `node` and pass the path to the
compiled service script as a command-line argument.

1.  **Start the Client and Connect to the Weather Service:**
    Use the following command from the repository root:

    ```bash
    node _bazel_bin/mcp/client/index.js _bazel_bin/mcp/services/weather/index.js
    ```

    - The first argument (`_bazel_bin/mcp/client/index.js`) is the path to the compiled
      client.
    - The second argument (`_bazel_bin/mcp/services/weather/index.js`) is the path to the
      compiled MCP service you want to connect to.

2.  **Interact with the Client:**
    Once running, the client will display a list of tools available from the connected
    service. You can then type queries for the model to answer. For example:

    ```
    Query: What is the weather like in New York?
    ```

    The client will use the Gemini API and the connected service's tools to respond. To
    exit, type `quit`.

    **Note:** The `get-forecast` tool requires latitude and longitude as input. For a more
    seamless experience, you can connect the client to a search MCP server that can
    provide these coordinates for a given location.
