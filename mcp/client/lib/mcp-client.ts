import { FunctionDeclarationSchema, Tool } from '@google/generative-ai';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { ServerConfig } from './settings';
import { SSEClientTransport } from '@modelcontextprotocol/sdk/client/sse.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

export class MCPClient {
  private client: Client;

  private transport: StdioClientTransport | SSEClientTransport | StreamableHTTPClientTransport;

  private serverName!: string; // Will be set in connect

  private toolsForModel: Tool[] = [];

  private prefixedSanitizedNameToOriginalNameMap: Map<string, string> = new Map();

  constructor() {
    // Initialize with a default client, will be re-initialized in connect
    this.client = new Client({ name: 'mcp-client-default', version: '1.0.0' });
    this.transport = new StdioClientTransport({ command: '' }); // Default transport
  }

  /**
   * Connects to an MCP server and populates tools.
   * @param serverName - A unique name for this server connection.
   * @param serverConfig - Configuration for the server.
   */
  async connect(serverName: string, serverConfig: ServerConfig): Promise<void> {
    this.serverName = serverName;

    if (serverConfig.command) {
      this.transport = new StdioClientTransport({
        command: serverConfig.command,
        args: serverConfig.args || [],
      });
    } else if (serverConfig.url) {
      try {
        // Try StreamableHTTP first
        const initResponse = await fetch(serverConfig.url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Accept: 'application/json, text/event-stream', // Indicate preference for streaming
          },
          body: JSON.stringify({
            jsonrpc: '2.0',
            method: 'initialize', // Standard MCP handshake method
            params: {},
          }),
        });
        if (
          initResponse.ok &&
          initResponse.headers.get('content-type')?.includes('text/event-stream')
        ) {
          this.transport = new StreamableHTTPClientTransport(new URL(serverConfig.url));
        } else if (initResponse.ok) {
          // Fallback for non-streaming HTTP if server responds with JSON
          this.transport = new StreamableHTTPClientTransport(new URL(serverConfig.url)); // Or a simpler HTTP transport if available
        } else {
          throw new Error(
            `Streamable HTTP transport initialization failed with status: ${initResponse.status}`
          );
        }
      } catch (e) {
        console.warn(
          `StreamableHTTP transport failed for ${serverName}: ${
            e instanceof Error ? e.message : String(e)
          }. Falling back to SSE.`
        );
        this.transport = new SSEClientTransport(new URL(serverConfig.url));
      }
    } else {
      throw new Error(`Invalid server config for ${serverName}: No command or URL provided.`);
    }

    this.client = new Client({
      name: `mcp-client-cli-${this.serverName}`,
      version: '1.0.0',
    });

    try {
      await this.client.connect(this.transport);
    } catch (e) {
      console.error(`Failed to connect client for server ${this.serverName}:`, e);
      throw new Error(
        `Connection failed for ${this.serverName}: ${e instanceof Error ? e.message : String(e)}`
      );
    }

    try {
      const toolsResult = await this.client.listTools();
      if (!toolsResult || !Array.isArray(toolsResult.tools)) {
        console.warn(`No tools found or invalid format for server ${this.serverName}.`);
        this.toolsForModel = [];
        return;
      }

      const functionDeclarations = toolsResult.tools.map((tool) => {
        const originalToolName = tool.name;
        // Sanitize only the original tool name part
        const sanitizedOriginalToolName = originalToolName.replace(/[^a-zA-Z0-9_.-]/g, '_');
        // Prefix with server name for uniqueness in the model
        const modelFacingToolName = `${this.serverName}_${sanitizedOriginalToolName}`;

        this.prefixedSanitizedNameToOriginalNameMap.set(modelFacingToolName, originalToolName);

        if (modelFacingToolName !== `${this.serverName}_${originalToolName}`) {
          console.log(
            `Tool name sanitization for server "${this.serverName}":\n` +
              `  original "${originalToolName}" -> model_facing "${modelFacingToolName}"`
          );
        }

        // Ensure inputSchema exists and is an object, otherwise provide a default
        const inputSchema =
          typeof tool.inputSchema === 'object' && tool.inputSchema !== null
            ? tool.inputSchema
            : { type: 'object', properties: {} };

        const { additionalProperties, $schema, ...rest } = inputSchema as any;

        return {
          name: modelFacingToolName,
          description: tool.description || 'No description provided.',
          parameters: rest as FunctionDeclarationSchema,
        };
      });

      if (functionDeclarations.length > 0) {
        this.toolsForModel = [{ functionDeclarations }];
        console.log(
          `Connected to server "${this.serverName}" with tools (model-facing names):`,
          functionDeclarations.map((fd) => fd.name)
        );
      } else {
        this.toolsForModel = [];
        console.log(`No tools registered for server "${this.serverName}".`);
      }
    } catch (e) {
      console.error(`Failed to get tools from server ${this.serverName}:`, e);
      this.toolsForModel = []; // Ensure toolsForModel is empty on error
    }
  }

  /**
   * Gets the tools structured for the generative model, with names prefixed by the server name.
   * @returns An array of Tool objects.
   */
  getToolsForModel(): Tool[] {
    return this.toolsForModel;
  }

  /**
   * Gets the name of the server this client is connected to.
   * @returns The server name.
   */
  getServerName(): string {
    return this.serverName;
  }

  /**
   * Calls a tool on the MCP server.
   * @param functionCall - The function call object from the generative model.
   *                       `functionCall.name` is expected to be the server-prefixed, sanitized name.
   * @returns The result of the tool call.
   */
  async callTool(functionCall: { name: string; args: any }): Promise<any> {
    const modelFacingToolName = functionCall.name;
    const originalToolName = this.prefixedSanitizedNameToOriginalNameMap.get(modelFacingToolName);

    if (!originalToolName) {
      const errorMsg = `Original tool name not found for model-facing name "${modelFacingToolName}" on server "${this.serverName}". Check mapping.`;
      console.error(errorMsg);
      // It's important to return a structured error that the calling code (e.g., in main.ts) can handle
      return {
        error: {
          message: errorMsg,
          code: 'ToolNameMappingNotFound',
        },
      };
    }

    try {
      return await this.client.callTool({
        name: originalToolName,
        arguments: functionCall.args as { [k: string]: any },
      });
    } catch (e) {
      console.error(
        `Error calling tool "${originalToolName}" (model-facing: "${modelFacingToolName}") on server "${this.serverName}":`,
        e
      );
      return {
        error: {
          // Propagate a structured error
          message: e instanceof Error ? e.message : String(e),
          code: 'ToolExecutionError',
          originalToolName: originalToolName,
          modelFacingToolName: modelFacingToolName,
        },
      };
    }
  }

  /**
   * Cleans up the client connection.
   */
  async cleanup(): Promise<void> {
    if (this.client && typeof this.client.close === 'function') {
      try {
        await this.client.close();
        console.log(`MCPClient for server "${this.serverName}" closed.`);
      } catch (e) {
        console.error(`Error closing MCPClient for server "${this.serverName}":`, e);
      }
    }
  }
}
