import { expect } from 'chai';
import 'mocha';
import * as sinon from 'sinon';
import * as os from 'os';
import { readSettings, writeSettings, getSettingsFile } from './settings';

class MockFs {
  private store: { [key: string]: string } = {};

  async readFile(path: string, _encoding: string): Promise<string> {
    if (this.store[path]) {
      return this.store[path];
    }
    throw { code: 'ENOENT' };
  }

  async writeFile(path: string, data: string): Promise<void> {
    this.store[path] = data;
  }

  async mkdir(_path: string, _options: any): Promise<void> {
    // Do nothing.
  }
}

describe('Settings', () => {
  let mockFs: MockFs;

  beforeEach(() => {
    sinon.stub(os, 'homedir').returns('/home/test');
    mockFs = new MockFs();
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should return empty settings if the file does not exist', async () => {
    const result = await readSettings(mockFs);
    expect(result).to.deep.equal({ mcpServers: {} });
  });

  it('should read settings from a file', async () => {
    const settings = { mcpServers: { test: { command: 'test' } } };
    const settingsPath = getSettingsFile();
    await mockFs.writeFile(settingsPath, JSON.stringify(settings));
    const result = await readSettings(mockFs);
    expect(result).to.deep.equal(settings);
  });

  it('should throw an error if the file is malformed', async () => {
    const settingsPath = getSettingsFile();
    await mockFs.writeFile(settingsPath, 'not json');
    try {
      await readSettings(mockFs);
      // Should not reach here.
      expect.fail('Expected readSettings to throw an error for malformed JSON.');
    } catch (e: any) {
      expect(e.message).to.contain('Failed to read or parse settings file');
    }
  });

  it('should write settings to a file', async () => {
    const settings = { mcpServers: { test: { command: 'test' } } };
    await writeSettings(settings, mockFs);
    const settingsPath = getSettingsFile();
    const result = await mockFs.readFile(settingsPath, 'utf-8');
    expect(JSON.parse(result)).to.deep.equal(settings);
  });
});
