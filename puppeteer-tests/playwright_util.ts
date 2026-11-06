import * as fs from 'fs';
import * as path from 'path';
import { chromium, Browser, BrowserContext, Page } from 'playwright-core';

const ENV_PORT_FILE_BASE_NAME = 'port';
const DEFAULT_HEADFUL_CHROME_PATH = '/usr/bin/google-chrome';
const DEFAULT_ALLOWED_ERROR_PATTERNS: readonly RegExp[] = [/ResizeObserver loop/i];

export interface PlaywrightTestBed {
  page: Page;
  baseUrl: string;
}

export const launchPlaywrightBrowser = (
  showBrowser?: boolean,
  headfulFallbackPath: string = DEFAULT_HEADFUL_CHROME_PATH
): Promise<Browser> => {
  // Prefer CHROME_BIN_OVERRIDE when debugging locally, falling back to the
  // hermetic @rules_browsers toolchain binary provided by Bazel. Because
  // @rules_browsers provides chrome-headless-shell (which cannot render GUI
  // windows), fall back to /usr/bin/google-chrome when showBrowser is true.
  let executablePath = process.env.CHROME_BIN_OVERRIDE || process.env.CHROME_BIN;
  if (
    showBrowser &&
    !process.env.CHROME_BIN_OVERRIDE &&
    executablePath?.includes('chrome-headless-shell')
  ) {
    if (!fs.existsSync(headfulFallbackPath)) {
      throw new Error(
        `Cannot launch headful browser with chrome-headless-shell and ${headfulFallbackPath} was not found. Set CHROME_BIN_OVERRIDE to a headful Chrome binary path.`
      );
    }
    executablePath = headfulFallbackPath;
  }
  const slowMo = process.env.SLOW_MO ? parseInt(process.env.SLOW_MO, 10) : undefined;
  return chromium.launch({
    executablePath,
    slowMo,
    headless: !showBrowser,
    // Docker and Bazel sandboxes restrict shared memory and lack SUID sandboxes.
    args: ['--disable-dev-shm-usage', '--no-sandbox'],
  });
};

let browser: Browser | undefined;
let context: BrowserContext | undefined;
let testBed: Partial<PlaywrightTestBed>;
let hooksRegistered = false;
let resolvedShowBrowser = false;
let uncaughtPageErrors: string[] = [];
const suiteAllowedErrorPatterns: RegExp[] = [];

/**
 * Registers additional regex patterns of uncaught page errors that should not
 * fail the active test in afterEach.
 */
export function allowPageErrors(...patterns: RegExp[]): void {
  suiteAllowedErrorPatterns.push(...patterns);
}

function isAllowedPageError(msg: string): boolean {
  return (
    DEFAULT_ALLOWED_ERROR_PATTERNS.some((re) => re.test(msg)) ||
    suiteAllowedErrorPatterns.some((re) => re.test(msg))
  );
}

function sanitizeTitle(title: string): string {
  return title
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 120);
}

export async function loadPlaywrightTestBed(showBrowser?: boolean): Promise<PlaywrightTestBed> {
  if (typeof showBrowser === 'undefined') {
    showBrowser = Boolean(
      process.env.PLAYWRIGHT_TEST_SHOW_BROWSER || process.env.PUPPETEER_TEST_SHOW_BROWSER
    );
  }
  resolvedShowBrowser = showBrowser;

  if (testBed) {
    if (!browser || !browser.isConnected()) {
      browser = await launchPlaywrightBrowser(resolvedShowBrowser);
    }
    if (!hooksRegistered) {
      registerMochaHooks();
    }
    return testBed as PlaywrightTestBed;
  }
  const envDir = process.env.ENV_DIR;
  if (!envDir) {
    throw new Error('required environment variable ENV_DIR is unset');
  }
  const port = parseInt(
    fs.readFileSync(path.join(envDir, ENV_PORT_FILE_BASE_NAME), 'utf8').trim(),
    10
  );
  const newTestBed: Partial<PlaywrightTestBed> = {
    baseUrl: `http://localhost:${port}`,
  };

  browser = await launchPlaywrightBrowser(resolvedShowBrowser);
  testBed = newTestBed;
  registerMochaHooks();
  return testBed as PlaywrightTestBed;
}

export function registerMochaHooks(): void {
  if (hooksRegistered) {
    return;
  }
  hooksRegistered = true;

  beforeEach(async function () {
    if (
      process.env.SLOW_MO ||
      process.env.PLAYWRIGHT_TEST_SHOW_BROWSER ||
      process.env.PUPPETEER_TEST_SHOW_BROWSER
    ) {
      this.timeout(0);
    }
    uncaughtPageErrors = [];
    if (!browser || !browser.isConnected()) {
      browser = await launchPlaywrightBrowser(resolvedShowBrowser);
    }
    context = await browser.newContext();
    await context.tracing.start({ screenshots: true, snapshots: true });
    // Demo pages check this cookie to suppress animations and non-deterministic latency.
    await context.addCookies([
      {
        url: testBed.baseUrl!,
        name: 'puppeteer',
        value: 'true',
      },
    ]);
    const page = await context.newPage();
    page.on('pageerror', (err: Error) => {
      const msg = err?.stack || err?.message || String(err);
      if (!isAllowedPageError(msg)) {
        uncaughtPageErrors.push(msg);
      }
    });
    testBed.page = page;
  });

  afterEach(async function () {
    const failed = this.currentTest?.state === 'failed' || uncaughtPageErrors.length > 0;
    const saveTrace = failed || Boolean(process.env.SAVE_TRACE);
    const outputsDir = process.env.TEST_UNDECLARED_OUTPUTS_DIR;

    if (saveTrace && outputsDir && context) {
      const testTitle =
        this.currentTest?.fullTitle?.() || this.currentTest?.title || 'unknown_test';
      const artifactDir = path.join(outputsDir, 'playwright', sanitizeTitle(testTitle));
      const screenshotName = failed ? 'failure.png' : 'screenshot.png';
      try {
        fs.mkdirSync(artifactDir, { recursive: true });
        if (testBed.page && !testBed.page.isClosed()) {
          await testBed.page.screenshot({
            path: path.join(artifactDir, screenshotName),
            fullPage: true,
          });
        }
        await context.tracing.stop({ path: path.join(artifactDir, 'trace.zip') });
      } catch {
        // Ignore secondary artifact errors during teardown.
      }
    } else if (context) {
      try {
        await context.tracing.stop();
      } catch {
        // Ignore if tracing was already stopped.
      }
    }

    if (context) {
      await context.close();
      context = undefined;
    }
    if (testBed) {
      testBed.page = undefined;
    }

    if (uncaughtPageErrors.length > 0) {
      const joined = uncaughtPageErrors.join('\n---\n');
      uncaughtPageErrors = [];
      throw new Error(`Uncaught pageerror during test:\n${joined}`);
    }
  });

  after(async () => {
    if (browser && browser.isConnected()) {
      await browser.close();
    }
    browser = undefined;
  });
}
