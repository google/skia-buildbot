import readline from 'readline/promises';
import { MCPClient } from '../lib/mcp-client';
import { readSettings } from '../lib/settings';
import dotenv from 'dotenv';

dotenv.config();

async function main() {
  const mcpClient = new MCPClient(process.env.GEMINI_API_KEY as string);
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  try {
    const settings = await readSettings();
    const serverNames = Object.keys(settings.mcpServers);
    if (serverNames.length === 0) {
      console.log('No servers configured. Please add servers to your settings file.');
      return;
    }

    await mcpClient.connectToServers(settings.mcpServers);
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
