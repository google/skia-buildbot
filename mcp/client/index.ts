import {
  GoogleGenerativeAI,
  FunctionDeclarationSchema,
  GenerativeModel,
  Tool,
  FinishReason,
} from '@google/generative-ai';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import readline from 'readline/promises';
import dotenv from 'dotenv';

dotenv.config();

process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled Rejection at:', promise, 'reason:', reason);
  // Application specific logging, throwing an error, or other logic here
});

const GEMINI_API_KEY = process.env.GEMINI_API_KEY;
if (!GEMINI_API_KEY) {
  throw new Error('GEMINI_API_KEY is not set');
}

class MCPClient {
  private mcp: Client;

  private genAI: GoogleGenerativeAI;

  private transport: StdioClientTransport | null = null;

  private tools: Tool[] = [];

  private model: GenerativeModel | null = null;

  constructor() {
    this.genAI = new GoogleGenerativeAI(GEMINI_API_KEY as string);
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

async function main() {
  if (process.argv.length < 3) {
    console.log('Usage: node index.ts <path_to_server_script>');
    return;
  }
  const mcpClient = new MCPClient();
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  try {
    await mcpClient.connectToServer(process.argv[2]);
    console.log('\nMCP Client Started!');
    console.log("Type your queries or 'quit' to exit.");

    while (true) {
      try {
        const message = await rl.question('\nQuery: ');
        if (message.toLowerCase() === 'quit') {
          break;
        }
        const response = await mcpClient.processQuery(message);
        console.log('\n' + response);
      } catch (e) {
        console.error('Error in chat loop: ', e);
      }
    }
  } catch (e) {
    console.error('An unexpected error occurred: ', e);
  } finally {
    rl.close();
    await mcpClient.cleanup();
  }
}

main();
