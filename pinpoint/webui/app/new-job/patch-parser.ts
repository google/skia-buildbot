export interface Patch {
  host: string;
  change: number;
  patchset?: number;
}

const INTERNAL_GERRIT_DOMAIN = '.git.corp.google.com';
const PUBLIC_GERRIT_DOMAIN = '.googlesource.com';
const CHROMIUM_REVIEW_HOST = `https://chromium-review${PUBLIC_GERRIT_DOMAIN}`;

export function parsePatch(input: string): Patch {
  input = input.trim();
  if (!input) {
    throw new Error('Could not parse an empty string');
  }

  const shortcutResult = parsePatchShortcut(input);
  if (shortcutResult) {
    return shortcutResult;
  }

  const crrevResult = parseCrrev(input);
  if (crrevResult) {
    return crrevResult;
  }

  const gerritResult = parseGerritUrl(input);
  if (gerritResult) {
    return gerritResult;
  }

  throw new Error(`Could not parse patch: "${input}".`);
}

function parsePatchShortcut(input: string): Patch | null {
  if (!/^\d+(\/\d+)?$/.test(input)) {
    return null;
  }
  return {
    host: CHROMIUM_REVIEW_HOST,
    ...parseGerritPath('/' + input),
  };
}

function parseCrrev(input: string): Patch | null {
  const match = input.match(/^(https?:\/\/)?(www\.)?crrev(?:\.com)?\/(.*)$/i);
  if (!match) {
    return null;
  }
  try {
    return {
      host: CHROMIUM_REVIEW_HOST,
      ...parseGerritPath('/' + match[3]),
    };
  } catch (e: any) {
    throw new Error(`Invalid crrev link: ${e.message}`);
  }
}

function parseGerritUrl(input: string): Patch | null {
  if (!/^https?:\/\//i.test(input)) {
    input = 'https://' + input;
  }
  const url = new URL(input);
  if (url.protocol !== 'https:') {
    throw new Error('HTTP protocol is not allowed. Please use HTTPS.');
  }
  if (!url.host.includes(INTERNAL_GERRIT_DOMAIN) && !url.host.includes(PUBLIC_GERRIT_DOMAIN)) {
    throw new Error(`Invalid host "${url.host}"`);
  }
  return {
    host: url.protocol + '//' + url.host,
    ...parseGerritPath(url.pathname),
  };
}

function parseGerritPath(path: string): { change: number; patchset?: number } {
  const match = path.match(/\/(\d+)(?:\/(\d+))?(?:\/[^/0-9]+)?\/?$/);
  if (!match) {
    throw new Error(`Could not find change ID in path "${path}"`);
  }

  const change = parseInt(match[1], 10);
  if (change <= 0) {
    throw new Error(`Invalid Change ID: ${change}. Must be greater than 0.`);
  }

  if (!match[2]) {
    return { change };
  }

  const patchset = parseInt(match[2], 10);
  if (patchset <= 0) {
    throw new Error(`Invalid Patchset: ${patchset}. Must be greater than 0.`);
  }
  return { change, patchset };
}
