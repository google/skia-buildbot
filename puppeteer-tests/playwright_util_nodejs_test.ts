import * as fs from 'fs';
import * as http from 'http';
import * as net from 'net';
import * as os from 'os';
import * as path from 'path';
import express from 'express';
import { expect } from 'chai';
import {
  allowPageErrors,
  launchPlaywrightBrowser,
  loadPlaywrightTestBed,
  PlaywrightTestBed,
} from './playwright_util';

describe('Playwright test runner utilities', () => {
  let server: http.Server;
  let envDir: string;
  let outputsDir: string;
  let testBed: PlaywrightTestBed;
  let savedEnvDir: string | undefined;
  let savedOutputsDir: string | undefined;
  let savedSaveTrace: string | undefined;

  // Register a suite-level allowed page error pattern before tests run.
  allowPageErrors(/ExpectedSuiteLevelError/);

  before(async () => {
    savedEnvDir = process.env.ENV_DIR;
    savedOutputsDir = process.env.TEST_UNDECLARED_OUTPUTS_DIR;
    savedSaveTrace = process.env.SAVE_TRACE;

    const app = express();
    app.get('/', (_req, res) => {
      res.send(`
        <!DOCTYPE html>
        <html>
          <body>
            <h1>Playwright Test Page</h1>
          </body>
        </html>
      `);
    });
    await new Promise<void>((resolve) => {
      server = app.listen(0, () => resolve());
    });
    const port = (server.address() as net.AddressInfo).port;

    envDir = fs.mkdtempSync(path.join(os.tmpdir(), 'playwright-env-'));
    outputsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'playwright-out-'));
    fs.writeFileSync(path.join(envDir, 'port'), String(port), 'utf8');

    process.env.ENV_DIR = envDir;
    process.env.TEST_UNDECLARED_OUTPUTS_DIR = outputsDir;
    process.env.SAVE_TRACE = '1';

    testBed = await loadPlaywrightTestBed();
  });

  after((done) => {
    if (savedEnvDir === undefined) {
      delete process.env.ENV_DIR;
    } else {
      process.env.ENV_DIR = savedEnvDir;
    }
    if (savedOutputsDir === undefined) {
      delete process.env.TEST_UNDECLARED_OUTPUTS_DIR;
    } else {
      process.env.TEST_UNDECLARED_OUTPUTS_DIR = savedOutputsDir;
    }
    if (savedSaveTrace === undefined) {
      delete process.env.SAVE_TRACE;
    } else {
      process.env.SAVE_TRACE = savedSaveTrace;
    }
    fs.rmSync(envDir, { recursive: true, force: true });
    fs.rmSync(outputsDir, { recursive: true, force: true });
    server.close(done);
  });

  it('launches browser, sets puppeteer cookie, and ignores suite-registered page errors on test 1', async () => {
    await testBed.page.goto(testBed.baseUrl);
    expect(await testBed.page.locator('h1').innerText()).to.equal('Playwright Test Page');

    const cookie = await testBed.page.evaluate(() => document.cookie);
    expect(cookie).to.include('puppeteer=true');

    await testBed.page.evaluate(() => {
      setTimeout(() => {
        throw new Error('ResizeObserver loop completed with undelivered notifications.');
      }, 0);
      setTimeout(() => {
        throw new Error('ExpectedSuiteLevelError: safe to ignore');
      }, 0);
    });
    await testBed.page.waitForTimeout(20);
  });

  it('preserves suite-registered allowPageErrors across subsequent tests and writes trace artifacts', async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.evaluate(() => {
      setTimeout(() => {
        throw new Error('ExpectedSuiteLevelError: still safe on second test');
      }, 0);
    });
    await testBed.page.waitForTimeout(20);

    // Verify artifacts from the previous test were written to TEST_UNDECLARED_OUTPUTS_DIR.
    const playwrightOutDir = path.join(outputsDir, 'playwright');
    expect(fs.existsSync(playwrightOutDir)).to.equal(true);
    const subdirs = fs.readdirSync(playwrightOutDir);
    expect(subdirs.length).to.be.greaterThan(0);
    const firstArtifactDir = path.join(playwrightOutDir, subdirs[0]);
    expect(fs.existsSync(path.join(firstArtifactDir, 'screenshot.png'))).to.equal(true);
    expect(fs.existsSync(path.join(firstArtifactDir, 'trace.zip'))).to.equal(true);
  });

  it('throws an informative error when headful mode cannot find a headful Chrome binary', () => {
    const origChromeBin = process.env.CHROME_BIN;
    const origOverride = process.env.CHROME_BIN_OVERRIDE;
    try {
      process.env.CHROME_BIN = '/cache/browsers/chrome-headless-shell';
      delete process.env.CHROME_BIN_OVERRIDE;
      expect(() => launchPlaywrightBrowser(true, '/nonexistent/google-chrome')).to.throw(
        /CHROME_BIN_OVERRIDE/
      );
    } finally {
      if (origChromeBin === undefined) {
        delete process.env.CHROME_BIN;
      } else {
        process.env.CHROME_BIN = origChromeBin;
      }
      if (origOverride === undefined) {
        delete process.env.CHROME_BIN_OVERRIDE;
      } else {
        process.env.CHROME_BIN_OVERRIDE = origOverride;
      }
    }
  });
});
