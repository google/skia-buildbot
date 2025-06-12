import {
  GoogleGenerativeAI,
  FunctionDeclarationSchema,
  GenerativeModel,
  Tool,
  FinishReason,
} from '@google/generative-ai';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

export class MCPClient {
  private mcp: Client;

  private genAI: GoogleGenerativeAI;

  private transport: StdioClientTransport | null = null;

  private tools: Tool[] = [];

  private model: GenerativeModel | null = null;

  constructor(apiKey: string) {
    if (!apiKey) {
      throw new Error('GEMINI_API_KEY is not set');
    }
    this.genAI = new GoogleGenerativeAI(apiKey);
    this.mcp = new Client({ name: 'mcp-client-cli', version: '1.0.0' });
  }

  async connectToServer(serverScriptPath: string) {
    try {
      const isJs = serverScriptPath.endsWith('.js');
      const isPy = serverScriptPath.endsWith('.py');
      if (!isJs && !isPy) {
        throw new Error('Server script must be a .js or .py file');
      }
      const command = isPy
        ? process.platform === 'win32'
          ? 'python'
          : 'python3'
        : process.execPath;

      this.transport = new StdioClientTransport({
        command,
        args: [serverScriptPath],
      });
      this.mcp.connect(this.transport);

      const toolsResult = await this.mcp.listTools();
      const functionDeclarations = toolsResult.tools.map((tool) => {
        const { additionalProperties, $schema, ...rest } = tool.inputSchema as any;
        return {
          name: tool.name,
          description: tool.description,
          parameters: rest as FunctionDeclarationSchema,
        };
      });
      this.tools = [{ functionDeclarations }];
      this.model = this.genAI.getGenerativeModel({
        model: 'gemini-2.5-pro-preview-06-05',
        tools: this.tools,
      });

      console.log(
        'Connected to server with tools:',
        functionDeclarations.map((name: any) => name.name)
      );
    } catch (e) {
      console.log('Failed to connect to MCP server: ', e);
      throw e;
    }
  }

  async processQuery(query: string) {
    if (!this.model) {
      throw new Error('Model is not initialized. Call connectToServer first.');
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
            const result = await this.mcp.callTool({
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
    await this.mcp.close();
  }
}
