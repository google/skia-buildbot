import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';

const HADES_DIR = '.hades';
const SETTINGS_FILE = 'settings.json';

export interface ServerConfig {
  command?: string;
  args?: string[];
  url?: string;
}

export interface Settings {
  mcpServers: { [key: string]: ServerConfig };
}

export function getSettingsFile(): string {
  return path.join(os.homedir(), HADES_DIR, SETTINGS_FILE);
}

export async function readSettings(fsOverride?: any): Promise<Settings> {
  const fsPromises = fsOverride || fs;
  const settingsPath = getSettingsFile();
  try {
    const data = await fsPromises.readFile(settingsPath, 'utf-8');
    return JSON.parse(data) as Settings;
  } catch (e: any) {
    if (e.code === 'ENOENT') {
      // If the file doesn't exist, return empty settings.
      return { mcpServers: {} };
    }
    throw new Error(`Failed to read or parse settings file at ${settingsPath}: ${e}`);
  }
}

export async function writeSettings(settings: Settings, fsOverride?: any): Promise<void> {
  const fsPromises = fsOverride || fs;
  const settingsPath = getSettingsFile();
  try {
    await fsPromises.mkdir(path.dirname(settingsPath), { recursive: true });
    await fsPromises.writeFile(settingsPath, JSON.stringify(settings, null, 2));
  } catch (e) {
    throw new Error(`Failed to write settings to ${settingsPath}: ${e}`);
  }
}
