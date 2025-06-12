import readline from 'readline/promises';
import { MCPClient } from '../lib/mcp-client';
import dotenv from 'dotenv';

dotenv.config();

async function main() {
  if (process.argv.length < 3) {
    console.log('Usage: node main.ts <path_to_server_script>');
    return;
  }
  const mcpClient = new MCPClient(process.env.GEMINI_API_KEY as string);
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
