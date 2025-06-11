import {
  GoogleGenerativeAI,
  FunctionDeclarationSchema,
  GenerativeModel,
  Tool,
  FinishReason,
} from '@google/generative-ai';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { ServerConfig } from './settings';

export class MCPClient {
  private clients: { [serverName: string]: Client } = {};

  private transports: { [serverName: string]: StdioClientTransport } = {};

  private genAI: GoogleGenerativeAI;

  private tools: Tool[] = [];

  model: GenerativeModel | null = null;

  constructor(apiKey: string) {
    if (!apiKey) {
      throw new Error('GEMINI_API_KEY is not set');
    }
    this.genAI = new GoogleGenerativeAI(apiKey);
  }

  async connectToServers(servers: { [serverName: string]: ServerConfig }) {
    const allFunctionDeclarations: any[] = [];

    for (const serverName in servers) {
      const serverConfig = servers[serverName];
      try {
        const command = serverConfig.command;
        if (!command) {
          console.log(`Skipping server ${serverName} because it has no command.`);
          continue;
        }

        const transport = new StdioClientTransport({
          command,
          args: serverConfig.args || [],
        });
        this.transports[serverName] = transport;

        const client = new Client({ name: `mcp-client-cli-${serverName}`, version: '1.0.0' });
        this.clients[serverName] = client;
        client.connect(transport);

        const toolsResult = await client.listTools();
        const functionDeclarations = toolsResult.tools.map((tool) => {
          const { additionalProperties, $schema, ...rest } = tool.inputSchema as any;
          return {
            name: tool.name,
            description: tool.description,
            parameters: rest as FunctionDeclarationSchema,
          };
        });
        allFunctionDeclarations.push(...functionDeclarations);

        console.log(
          `Connected to server ${serverName} with tools:`,
          functionDeclarations.map((tool: any) => tool.name)
        );
      } catch (_e) {
        // Failures are expected if the server is not running.
      }
    }

    this.tools = [{ functionDeclarations: allFunctionDeclarations }];
    this.model = this.genAI.getGenerativeModel({
      model: 'gemini-2.5-pro-preview-06-05',
      tools: this.tools,
    });
  }

  async processQuery(query: string) {
    if (!this.model) {
      throw new Error('Model is not initialized. Call connectToServers first.');
    }
    try {
      const chat = this.model.startChat();
      const result = await chat.sendMessage(query);
      const response = result.response;

      if (
        response.promptFeedback?.blockReason ||
        response.candidates?.[0]?.finishReason !== FinishReason.STOP
      ) {
        return 'Response was blocked for some reason.';
      }

      const functionCalls = response.functionCalls();
      if (functionCalls && functionCalls.length > 0) {
        console.log(`[Calling tools: ${functionCalls.map((fc) => fc.name).join(', ')}]`);
        const toolResults = await Promise.all(
          functionCalls.map(async (functionCall) => {
            let clientForTool: Client | null = null;
            for (const serverName in this.clients) {
              const client = this.clients[serverName];
              const tools = await client.listTools();
              if (tools.tools.some((tool) => tool.name === functionCall.name)) {
                clientForTool = client;
                break;
              }
            }

            if (!clientForTool) {
              throw new Error(`Could not find a client for tool ${functionCall.name}`);
            }

            const result = await clientForTool.callTool({
              name: functionCall.name,
              arguments: functionCall.args as { [k: string]: any },
            });
            return {
              functionResponse: {
                name: functionCall.name,
                response: {
                  name: functionCall.name,
                  content: result.content,
                },
              },
            };
          })
        );

        const finalResult = await chat.sendMessage(toolResults);
        return finalResult.response.text();
      } else {
        return response.text();
      }
    } catch (e) {
      console.error('Error processing query: ', e);
      return 'An error occurred while processing your query.';
    }
  }

  async cleanup() {
    for (const serverName in this.clients) {
      await this.clients[serverName].close();
    }
  }
}
