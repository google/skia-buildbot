# Weather Service

This document describes how to build and integrate the Weather Service with an MCP host
like the Gemini CLI. This guide is based on the general principles outlined in the
[MCP Server Quickstart](https://modelcontextprotocol.io/quickstart/server#node).

## Prerequisites

- Node.js and npm installed.
- TypeScript installed (globally or as a dev dependency, e.g., `npm install --save-dev typescript`).

## Building the Service

1.  **Install Dependencies:**
    From the root of the repository, run:

    ```bash
    npm ci
    ```

2.  **Build the Service:**
    Compile the TypeScript code using Bazel:
    ```bash
    bazelisk build //mcp/services/weather:index_ts_lib
    ```
    This command will compile the TypeScript code and place the output in the
    `_bazel_bin/mcp/services/weather/index.js` file.

## Running the Service

After building, the service is typically invoked directly by the MCP host (e.g., Gemini CLI)
as a command-line application when a tool from this service is called. The `package.json`
may define a `bin` entry (e.g., `"weather": "./build/index.js"`) that specifies the
executable script.

No separate, persistent "running" step is usually needed for the server itself if it's
designed to be invoked on-demand by the CLI. The CLI will execute the command specified
in its configuration.

## Integrating with Gemini CLI

To integrate the Weather Service with the Gemini CLI when it's run as a local command:

1.  **Ensure the service is built:** The compiled script (e.g., `build/index.js`) must be present.
2.  **Configure Gemini CLI:**
    You'll need to configure the Gemini CLI to invoke your service as a local command.
    This is typically done in a Gemini CLI settings file (e.g., often found at
    `~/.gemini/settings.json` or a project-specific configuration).

        Based on your provided configuration structure, an entry for the weather service would

    look like this:

        ```json
        // Example structure in settings.json
        {
          "mcpServers": {
            "weather": {
              "command": "node",
              "args": [
                "/usr/local/google/home/jewoo/code/buildbot/_bazel_bin/mcp/services/weather/index.js"
              ]
            }
            // ... other services like "chrome_infra" might be defined here
          },
          "theme": "Default" // Or your current theme
        }
        ```

        Make sure the path in the `args` array correctly points to the compiled `index.js`

    file of your weather service.

        **Note:** After updating the configuration, you might need to restart or reload the

    Gemini CLI for changes to take effect.

## Service Tools

This service provides the following tools, which will become available in the Gemini CLI
after successful integration:

- `get-forecast`: Retrieves the weather forecast for a given latitude and longitude.
  - Parameters: `latitude` (float), `longitude` (float)
- `get-alerts`: Retrieves weather alerts for a given two-letter state code.
  - Parameters: `state` (string, e.g., CA, NY)

For more details on the service's implementation and how it exposes these tools via
the Model Context Protocol, refer to its source code (e.g., `src/index.ts`).
