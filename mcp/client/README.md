# MCP Client

This document describes the MCP client, which consists of a reusable library and a CLI.

## Library

The core of the MCP client is a reusable library located in `mcp/client/lib`. This
library provides a simple way to connect to an MCP server, discover its tools, and
process queries using the Gemini API.

### Usage

To use the library, import the `MCPClient` class and create a new instance. Then, call
the `connectToServers` method with the server configurations.

```typescript
import { MCPClient } from './lib/mcp-client';
import { readSettings } from './lib/settings';

const client = new MCPClient('YOUR_API_KEY_HERE');
const settings = await readSettings();
await client.connectToServers(settings.mcpServers);
const response = await client.processQuery('What is the weather like in New York?');
console.log(response);
```

## CLI

The CLI provides a simple REPL to interact with the MCP.

### Prerequisites

- Node.js and npm installed.
- A Google Gemini API Key.

### Setup

1.  **Install Dependencies:**
    From the root of the repository, run the following command to install the required
    Node.js packages:

    ```bash
    npm install
    ```

2.  **Configure API Key:**
    Create a `.env` file in the root of the repository. Add your Gemini API key to this
    file:

    ```
    GEMINI_API_KEY="YOUR_API_KEY_HERE"
    ```

    The client uses this file to load your API key securely.

3.  **Configure Servers:**
    The client reads a list of MCP servers to connect to from `~/.hades/settings.json`.
    Create this file with the following format:

    ```json
    {
      "mcpServers": {
        "weather": {
          "command": "node",
          "args": ["_bazel_bin/mcp/services/weather/index.js"]
        },
        "mainServer": {
          "command": "bin/mcp_server.py"
        },
        "anotherServer": {
          "url": "http://another-mcp-server.com"
        }
      }
    }
    ```

    The `mcpServers` object contains a map of server names to server configurations.
    Each server configuration can have one of the following forms:

    - A `url` for a remote server.
    - A `command` for a local server.
    - A `command` and `args` for a local server with arguments.

### Building

The client and any services it connects to must be compiled using Bazel.

1.  **Build the Client:**

    ```bash
    bazelisk build //mcp/client/cli:main_ts_lib
    ```

2.  **Build the Weather Service:**
    To test the client, build the example weather service:
    ```bash
    bazelisk build //mcp/services/weather:index_ts_lib
    ```
    These commands compile the TypeScript code and place the output in the `_bazel_bin`
    directory.

### Running the Client

To run the client, you execute its compiled script with `node`.

1.  **Start the Client:**
    Use the following command from the repository root:

    ```bash
    node _bazel_bin/mcp/client/cli/main.js
    ```

    The client will prompt you to select a server if multiple servers are configured in
    your `settings.json` file.

2.  **Interact with the Client:**
    Once running, the client will display a list of tools available from the connected
    service. You can then type queries for the model to answer. For example:

    ```
    Query: What is the weather like in New York?
    ```

    The client will use the Gemini API and the connected service's tools to respond. To
    exit, type `quit`.

    **Note:** The `get-forecast` tool requires latitude and longitude as input. For a
    more seamless experience, you can connect the client to a search MCP server that can
    provide these coordinates for a given location.
