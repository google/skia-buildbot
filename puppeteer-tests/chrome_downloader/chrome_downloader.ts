/**
 * This program hermetically downloads Chromium, mimicking (i.e. reusing code from) Puppeteer. See
 * notes in //.puppeteerrc.json for additional context.
 */

import { Browser, install } from '@puppeteer/browsers';
import { join, resolve } from 'path';
import { CHROME_EXECUTABLE_PATH } from './chrome_executable_path';
import { PUPPETEER_REVISIONS } from 'puppeteer-core/lib/esm/puppeteer/revisions.js';

const INSTALL_TIMEOUT_MS = 30_000; // Chosen arbitrarily; this usually takes ~10 seconds.

async function installChrome(downloadDir: string) {
  console.log(`Downloading Chrome to: ${downloadDir}`);

  // For some reason, the install() function occasionally takes forever to uncompress the
  // downloaded archive, and Bazel waits indefinitely for //puppeteer-test:chrome to finish.
  const timeout = setTimeout(() => {
    console.log(
      `Download/uncompress took more than ${INSTALL_TIMEOUT_MS} ms; aborting.`
    );
    process.exit(1);
  }, INSTALL_TIMEOUT_MS);

  // Report download progress in 10% increments.
  let lastPct = Number.MIN_VALUE;
  const progressCallback = (downloadedBytes: number, totalBytes: number) => {
    const pct = Math.trunc((downloadedBytes / totalBytes) * 100);
    if (pct >= lastPct + 10) {
      console.log(
        `${pct}% downloaded (${downloadedBytes} / ${totalBytes} bytes)...`
      );
      lastPct = pct;
    }
  };

  const installedBrowser = await install({
    browser: Browser.CHROMIUM,
    buildId: PUPPETEER_REVISIONS.chromium,
    cacheDir: downloadDir,
    downloadProgressCallback: progressCallback,
  });
  clearTimeout(timeout);

  console.log(
    `Downloaded Chrome. Executable path: ${installedBrowser.executablePath}`
  );

  if (
    installedBrowser.executablePath !==
    join(downloadDir, CHROME_EXECUTABLE_PATH)
  ) {
    const prefixLength = downloadDir.length + 1; // Account for trailing slash.
    const actualExecutablePath =
      installedBrowser.executablePath.slice(prefixLength);
    console.error(
      "ERROR: The downloaded browser's executable path does not match the expected path."
    );
    console.error(
      `    Actual (relative to download directory): ${actualExecutablePath}`
    );
    console.error(
      `    Expected:                                ${CHROME_EXECUTABLE_PATH}`
    );
    process.exit(1);
  }
}

// The process.argv array looks like ["path/to/node", "program.js", "arg1", "arg2", ...].
if (process.argv.length !== 3) {
  console.error(
    `Expected exactly one command-line argument, got ${
      process.argv.length - 2
    }.`
  );
  process.exit(1);
}
const downloadDir = resolve(process.argv[2]);

installChrome(downloadDir);
