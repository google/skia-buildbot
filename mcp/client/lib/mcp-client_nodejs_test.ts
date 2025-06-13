import { MCPClient } from './mcp-client';
import { expect } from 'chai';
import 'mocha';
import * as sinon from 'sinon';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { SSEClientTransport } from '@modelcontextprotocol/sdk/client/sse.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

describe('MCPClient', () => {
  let mcpClient: MCPClient;
  let listToolsStub: sinon.SinonStub;
  let callToolStub: sinon.SinonStub;
  let closeStub: sinon.SinonStub;

  beforeEach(() => {
    sinon.stub(StdioClientTransport.prototype, 'close');
    sinon.stub(SSEClientTransport.prototype, 'close');
    sinon.stub(StreamableHTTPClientTransport.prototype, 'close');
    sinon.stub(Client.prototype, 'connect');
    listToolsStub = sinon.stub(Client.prototype, 'listTools');
    callToolStub = sinon.stub(Client.prototype, 'callTool');
    closeStub = sinon.stub(Client.prototype, 'close');

    mcpClient = new MCPClient();
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should be able to be instantiated', () => {
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(mcpClient).to.not.be.null;
  });

  it('should connect to the local server', async () => {
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connect('test.js', { command: 'test.js' });
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(listToolsStub.calledOnce).to.be.true;
  });

  it('should connect to the remote server', async () => {
    const fetchStub = sinon.stub(global, 'fetch');
    fetchStub.resolves({ ok: true } as Response);
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connect('test.com', { url: 'http://test.com' });
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(listToolsStub.calledOnce).to.be.true;
  });

  it('should fall back to sse transport', async () => {
    const fetchStub = sinon.stub(global, 'fetch');
    fetchStub.onFirstCall().resolves({ ok: false } as Response);
    fetchStub.onSecondCall().resolves({ ok: true } as Response);
    listToolsStub.resolves({ tools: [] });
    await mcpClient.connect('test.com', { url: 'http://test.com' });
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(listToolsStub.calledOnce).to.be.true;
  });

  it('should get tools', async () => {
    listToolsStub.resolves({
      tools: [
        {
          name: 'test-tool',
          description: 'A test tool',
          inputSchema: {},
        },
      ],
    });
    await mcpClient.connect('test.js', { command: 'test.js' });
    const tools = mcpClient.getToolsForModel();
    expect(tools).to.have.lengthOf(1);
    expect((tools[0] as any).functionDeclarations).to.have.lengthOf(1);
    expect((tools[0] as any).functionDeclarations?.[0].name).to.equal('test.js_test-tool');
  });

  it('should call a tool', async () => {
    callToolStub.resolves({ content: 'tool response' });
    // Simulate the mapping that connect() would establish
    // Need to access the internal map for testing, or mock connect more thoroughly.
    // For now, let's assume connect correctly populates the map.
    // We also need listToolsStub to be active for connect() to populate the map.
    listToolsStub.resolves({
      tools: [
        {
          name: 'test-tool', // Original name
          description: 'A test tool',
          inputSchema: {},
        },
      ],
    });
    await mcpClient.connect('test.js', { command: 'test.js' });

    // Call mcpClient.callTool with the model-facing name
    await mcpClient.callTool({ name: 'test.js_test-tool', args: {} });

    // Assert that the SDK's callTool (callToolStub) was called once
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(callToolStub.calledOnce).to.be.true;
    // And assert it was called with the original tool name
    expect(callToolStub.getCall(0).args[0]).to.deep.equal({
      name: 'test-tool', // Original name expected by the SDK client
      arguments: {},
    });
  });

  it('should cleanup', async () => {
    await mcpClient.connect('test.js', { command: 'test.js' });
    await mcpClient.cleanup();
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(closeStub.calledOnce).to.be.true;
  });
});
