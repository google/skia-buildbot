import { MCPClient } from './mcp-client';
import { expect } from 'chai';
import 'mocha';
import * as sinon from 'sinon';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { GoogleGenerativeAI, GenerativeModel, FinishReason } from '@google/generative-ai';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

describe('MCPClient', () => {
  let mcpClient: MCPClient;
  let listToolsStub: sinon.SinonStub;
  let callToolStub: sinon.SinonStub;
  let closeStub: sinon.SinonStub;
  let startChatStub: sinon.SinonStub;
  let sendMessageStub: sinon.SinonStub;

  beforeEach(() => {
    sinon.stub(StdioClientTransport.prototype, 'close');
    sinon.stub(Client.prototype, 'connect');
    listToolsStub = sinon.stub(Client.prototype, 'listTools');
    callToolStub = sinon.stub(Client.prototype, 'callTool');
    closeStub = sinon.stub(Client.prototype, 'close');

    sendMessageStub = sinon.stub().resolves({
      response: {
        functionCalls: () => [],
        text: () => 'response',
        promptFeedback: {
          blockReason: null,
        },
        candidates: [
          {
            finishReason: FinishReason.STOP,
          },
        ],
      },
    });
    startChatStub = sinon.stub().returns({
      sendMessage: sendMessageStub,
    });
    sinon.stub(GoogleGenerativeAI.prototype, 'getGenerativeModel').returns({
      startChat: startChatStub,
    } as unknown as GenerativeModel);

    mcpClient = new MCPClient('test-key');
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should be able to be instantiated', () => {
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(mcpClient).to.not.be.null;
  });

  it('should connect to the servers', async () => {
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connectToServers({ 'test.js': { command: 'test.js' } });
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(listToolsStub.calledOnce).to.be.true;
  });

  it('should process a query', async () => {
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connectToServers({ 'test.js': { command: 'test.js' } });
    const response = await mcpClient.processQuery('test query');
    expect(response).to.equal('response');
  });

  it('should call a tool', async () => {
    listToolsStub.resolves({
      tools: [
        {
          name: 'test-tool',
          description: 'A test tool',
          inputSchema: {},
        },
      ],
    });
    sendMessageStub.resolves({
      response: {
        functionCalls: () => [{ name: 'test-tool', args: {} }],
        text: () => 'response',
        promptFeedback: {
          blockReason: null,
        },
        candidates: [
          {
            finishReason: FinishReason.STOP,
          },
        ],
      },
    });
    callToolStub.resolves({ content: 'tool response' });
    await mcpClient.connectToServers({ 'test.js': { command: 'test.js' } });
    await mcpClient.processQuery('test query');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(callToolStub.calledOnce).to.be.true;
  });

  it('should cleanup', async () => {
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connectToServers({ 'test.js': { command: 'test.js' } });
    await mcpClient.cleanup();
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(closeStub.calledOnce).to.be.true;
  });
});
