import readline from 'readline/promises';
import { MCPClient } from '../lib/mcp-client';
import { readSettings } from '../lib/settings';
import dotenv from 'dotenv';
import { GoogleGenerativeAI, FinishReason, Tool, FunctionCall, Part } from '@google/generative-ai';

dotenv.config();

const MODEL_NAME = 'gemini-2.5-pro-preview-06-05';

// Module-scoped variables for cleanup
let rl: readline.Interface | null = null;
const mcpClients: MCPClient[] = [];
let isCleaningUp = false;

/**
 * Performs cleanup of resources and exits the process.
 * @param source - The source of the cleanup trigger (e.g., 'SIGINT', 'quit').
 * @param exitCode - The exit code for the process.
 */
async function performCleanupAndExit(source: string, exitCode: number = 0) {
  if (isCleaningUp) {
    return; // Cleanup already in progress
  }
  isCleaningUp = true;
  console.log(`\n[${source}] Initiating graceful shutdown...`);

  if (rl) {
    console.log('Closing readline interface...');
    rl.close(); // This will cause rl.question() to stop waiting for input
  }

  console.log('Cleaning up MCP clients...');
  const cleanupPromises = mcpClients.map((client) =>
    client.cleanup().catch((e) => {
      console.error(`Error during cleanup for client ${client.getServerName()}:`, e);
    })
  );
  await Promise.allSettled(cleanupPromises);

  console.log(`[${source}] Hades CLI exited. Exit code: ${exitCode}`);
  process.exit(exitCode);
}

// Register signal handlers
process.on('SIGINT', () => {
  performCleanupAndExit('SIGINT', 130); // Standard exit code for SIGINT
});

process.on('SIGTERM', () => {
  performCleanupAndExit('SIGTERM', 143); // Standard exit code for SIGTERM
});

async function main() {
  rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  try {
    const settings = await readSettings();
    const serverNames = Object.keys(settings.mcpServers);
    if (serverNames.length === 0) {
      console.log('No servers configured. Please add servers to your settings file.');
      await performCleanupAndExit('no_servers_config', 1);
      return; // Should be unreachable
    }

    console.log('Connecting to MCP servers...');
    for (const serverName of serverNames) {
      const client = new MCPClient();
      try {
        await client.connect(serverName, settings.mcpServers[serverName]);
        mcpClients.push(client); // Populate module-scoped array
      } catch (e) {
        console.error(
          `Failed to connect to server ${serverName}:`,
          e instanceof Error ? e.message : String(e)
        );
      }
    }

    if (mcpClients.length === 0) {
      console.log('No MCP servers could be connected. Exiting.');
      await performCleanupAndExit('no_servers_connected', 1);
      return; // Should be unreachable
    }

    const genAI = new GoogleGenerativeAI(process.env.GEMINI_API_KEY as string);
    const allToolsForModel: Tool[] = mcpClients.flatMap((client) => client.getToolsForModel());

    const hasDeclarableTools = allToolsForModel.some(
      (tool) =>
        'functionDeclarations' in tool &&
        tool.functionDeclarations &&
        tool.functionDeclarations.length > 0
    );

    if (!hasDeclarableTools) {
      console.warn(
        'No tools with function declarations available from any connected MCP server. The CLI will operate without tool functionality.'
      );
    }

    const model = genAI.getGenerativeModel({
      model: MODEL_NAME,
      tools: hasDeclarableTools ? allToolsForModel : undefined,
    });

    console.log('\nHades CLI Started!');
    console.log("Type your queries or 'quit' to exit.");

    const chat = model.startChat();

    while (true) {
      if (isCleaningUp) {
        // Check if cleanup has been initiated by a signal
        break;
      }

      let message = '';
      try {
        message = await rl.question('\nQuery: ');
      } catch (err: any) {
        // This catch handles errors from rl.question(), e.g., if rl is closed during await
        if (isCleaningUp) {
          // If cleanup is in progress (e.g., due to SIGINT), this is expected
          // console.log('Readline question interrupted by shutdown.');
          break; // Exit loop to allow cleanup to proceed
        }
        // If not cleaning up, this is an unexpected error with readline
        console.error('Error reading input:', err.message);
        await performCleanupAndExit('readline_error', 1);
        return; // Should be unreachable
      }

      if (message.toLowerCase() === 'quit') {
        break; // Normal exit from loop, finally block will handle cleanup
      }

      const result = await chat.sendMessage(message);
      const response = result.response;

      if (
        response.promptFeedback?.blockReason ||
        (response.candidates &&
          response.candidates[0]?.finishReason !== FinishReason.STOP &&
          String(response.candidates[0]?.finishReason) !== 'TOOL_CALLS')
      ) {
        console.log(
          'Response was blocked or did not finish as expected. Reason:',
          response.promptFeedback?.blockReason || response.candidates?.[0]?.finishReason
        );
        continue;
      }

      const functionCalls = response.functionCalls();
      if (functionCalls && functionCalls.length > 0) {
        console.log(`[Calling tools: ${functionCalls.map((fc) => fc.name).join(', ')}]`);

        const toolResultsPromises = functionCalls.map(async (functionCall: FunctionCall) => {
          const modelFacingToolName = functionCall.name;
          const serverNamePrefix = modelFacingToolName.substring(
            0,
            modelFacingToolName.indexOf('_')
          );

          const clientForTool = mcpClients.find((c) => c.getServerName() === serverNamePrefix);

          if (!clientForTool) {
            console.error(
              `Could not find a client for server prefix "${serverNamePrefix}" from tool "${modelFacingToolName}"`
            );
            return {
              functionResponse: {
                name: modelFacingToolName,
                response: {
                  name: modelFacingToolName,
                  content: JSON.stringify({
                    error: `Client not found for server prefix ${serverNamePrefix}`,
                  }),
                },
              },
            };
          }

          const toolCallResult = await clientForTool.callTool(functionCall);

          if (toolCallResult && toolCallResult.error) {
            console.error(
              `Error from tool ${modelFacingToolName} on server ${clientForTool.getServerName()}:`,
              toolCallResult.error
            );
            return {
              functionResponse: {
                name: modelFacingToolName,
                response: {
                  name: modelFacingToolName,
                  content: JSON.stringify({
                    error: toolCallResult.error.message || 'Tool execution failed',
                  }),
                },
              },
            };
          }

          return {
            functionResponse: {
              name: modelFacingToolName,
              response: {
                name: modelFacingToolName,
                content:
                  toolCallResult && toolCallResult.content !== undefined
                    ? toolCallResult.content
                    : JSON.stringify({}),
              },
            },
          };
        });

        const toolResponses = await Promise.all(toolResultsPromises);

        const finalResult = await chat.sendMessage(toolResponses as Part[]);
        console.log('\n' + finalResult.response.text());
      } else if (response.text) {
        console.log('\n' + response.text());
      } else {
        console.log(
          'No text response and no tool calls. Raw response:',
          JSON.stringify(response, null, 2)
        );
      }
    } // End of while loop
  } catch (e) {
    console.error(
      'An unexpected error occurred in main execution: ',
      e instanceof Error ? e.message : String(e),
      e
    );
    await performCleanupAndExit('main_catch_block', 1); // Ensures cleanup and exit
    return; // Should be unreachable
  } finally {
    // This finally block handles the 'quit' case or other normal loop terminations.
    // If a signal or an error already triggered cleanup and exit, isCleaningUp will be true.
    if (!isCleaningUp) {
      await performCleanupAndExit('main_finally_normal_exit', 0);
    }
  }
}

main().catch((e) => {
  // This is a top-level catch for unhandled promise rejections from main() itself.
  console.error('Critical unhandled error launching main:', e);
  if (!isCleaningUp) {
    performCleanupAndExit('toplevel_main_catch', 1);
  }
});
